package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrEmailTaken         = errors.New("email already taken")
	ErrRegistrationClosed = errors.New("registration is closed")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUserBanned         = errors.New("account is banned")
	ErrInviteRequired     = errors.New("valid invite code required")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrTOTPRequired       = errors.New("2FA verification required")
	ErrInvalidTOTPCode    = errors.New("invalid 2FA code")
	ErrTOTPAlreadyEnabled = errors.New("2FA is already enabled")
	ErrTOTPNotEnabled     = errors.New("2FA is not enabled")
	ErrSMTPNotConfigured  = errors.New("SMTP is not configured")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var reservedUsernames = map[string]bool{
	"everyone": true,
	"here":     true,
	"channel":  true,
	"admin":    true,
}

const (
	accessTokenLifetime  = 15 * time.Minute
	refreshTokenLifetime = 30 * 24 * time.Hour
	twoFATokenLifetime   = 5 * time.Minute
	resetTokenLifetime   = 1 * time.Hour
	verifyTokenLifetime  = 24 * time.Hour
	bcryptCost           = 12
	recoveryCodeCount    = 8
	userCacheTTL         = 2 * time.Minute
)

// SMTPConfig holds optional SMTP settings for sending emails.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// AuthTokens is the response returned after successful authentication.
type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// TwoFAChallenge is returned when login requires 2FA verification.
type TwoFAChallenge struct {
	Requires2FA bool   `json:"requires_2fa"`
	TwoFAToken  string `json:"two_fa_token"`
}

// TOTPSetupInfo is returned when setting up 2FA.
type TOTPSetupInfo struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

type cachedUser struct {
	user      db.User
	fetchedAt time.Time
}

type AuthService struct {
	Queries          *db.Queries
	jwtSecret        []byte
	smtpConfig       *SMTPConfig
	frontendURL      string
	openRegistration bool
	instanceName     string
	userCache        sync.Map // uuid.UUID -> cachedUser
}

func NewAuthService(queries *db.Queries, jwtSecret string, smtpConfig *SMTPConfig, frontendURL string) *AuthService {
	return &AuthService{
		Queries:      queries,
		jwtSecret:    []byte(jwtSecret),
		smtpConfig:   smtpConfig,
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		instanceName: "Den",
	}
}

// UserInfo is the JSON-serializable user profile returned by /api/me.
type UserInfo struct {
	ID            uuid.UUID `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name,omitempty"`
	IsAdmin       bool      `json:"is_admin"`
	TotpEnabled   bool      `json:"totp_enabled"`
	EmailVerified bool      `json:"email_verified"`
}

func UserInfoFromDB(user db.User) UserInfo {
	info := UserInfo{
		ID:            user.ID,
		Username:      user.Username,
		Email:         user.Email,
		IsAdmin:       user.IsAdmin,
		TotpEnabled:   user.TotpEnabled,
		EmailVerified: user.EmailVerified,
	}
	if user.DisplayName.Valid {
		info.DisplayName = user.DisplayName.String
	}
	return info
}

// IsSMTPConfigured returns whether SMTP email sending is available.
func (service *AuthService) IsSMTPConfigured() bool {
	return service.smtpConfig != nil
}

// Register creates a new user account.
func (service *AuthService) Register(ctx context.Context, email, username, password, inviteCode string) (*AuthTokens, bool, error) {
	// Validate inputs
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if !isValidEmail(email) {
		return nil, false, ErrInvalidInput
	}
	if !usernameRegex.MatchString(username) || len(username) == 0 || len(username) > 32 {
		return nil, false, ErrInvalidInput
	}
	if reservedUsernames[strings.ToLower(username)] {
		return nil, false, ErrInvalidInput
	}
	if len(password) < 8 {
		return nil, false, ErrWeakPassword
	}

	// Check if first user (auto-admin)
	userCount, countError := service.Queries.CountUsers(ctx)
	if countError != nil {
		return nil, false, countError
	}
	isFirstUser := userCount == 0

	// Enforce invite code when registration is closed
	if !isFirstUser && !service.openRegistration {
		if !service.ValidateInviteCode(ctx, inviteCode) {
			return nil, false, ErrInviteRequired
		}
	}

	// Hash password
	passwordHash, hashError := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if hashError != nil {
		return nil, false, fmt.Errorf("hashing password: %w", hashError)
	}

	// Create user
	user, createError := service.Queries.CreateUser(ctx, db.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: string(passwordHash),
		DisplayName:  sql.NullString{},
		IsAdmin:      isFirstUser,
	})
	if createError != nil {
		if isUniqueViolation(createError) {
			// Determine which field caused the violation
			if strings.Contains(createError.Error(), "email") {
				return nil, false, ErrEmailTaken
			}
			return nil, false, ErrUsernameTaken
		}
		return nil, false, createError
	}

	// Increment invite code use count
	if !isFirstUser && !service.openRegistration && inviteCode != "" {
		code, lookupError := service.Queries.GetInviteCodeByCode(ctx, inviteCode)
		if lookupError == nil {
			if incrementError := service.Queries.IncrementInviteCodeUseCount(ctx, code.ID); incrementError != nil {
				log.Printf("warning: failed to increment invite code use count: %v", incrementError)
			}
		}
	}

	// If SMTP is configured, send verification email instead of returning tokens
	if service.smtpConfig != nil {
		verifyToken, tokenError := generateRandomToken()
		if tokenError != nil {
			return nil, false, tokenError
		}
		tokenHash := hashToken(verifyToken)
		if setError := service.Queries.SetEmailVerifyToken(ctx, db.SetEmailVerifyTokenParams{
			ID:                   user.ID,
			EmailVerifyToken:     sql.NullString{String: tokenHash, Valid: true},
			EmailVerifyExpiresAt: sql.NullTime{Time: time.Now().Add(verifyTokenLifetime), Valid: true},
		}); setError != nil {
			return nil, false, setError
		}

		verifyURL := fmt.Sprintf("%s/verify-email?token=%s", service.frontendURL, verifyToken)
		emailBody := fmt.Sprintf(
			"<h2>Welcome to %s!</h2><p>Click the link below to verify your email address:</p><p><a href=\"%s\">Verify Email</a></p><p>This link expires in 24 hours.</p>",
			service.instanceName, verifyURL,
		)
		if sendError := service.sendEmail(email, "Verify your email", emailBody); sendError != nil {
			log.Printf("warning: failed to send verification email: %v", sendError)
		}
		return nil, true, nil // email_verification_required = true
	}

	// SMTP not configured — mark email as verified and issue tokens
	if verifyError := service.Queries.SetEmailVerified(ctx, user.ID); verifyError != nil {
		log.Printf("warning: failed to set email_verified: %v", verifyError)
	}
	user.EmailVerified = true

	tokens, tokenError := service.issueTokens(ctx, user)
	if tokenError != nil {
		return nil, false, tokenError
	}
	return tokens, false, nil
}

// Login authenticates a user with email and password.
func (service *AuthService) Login(ctx context.Context, email, password string) (*AuthTokens, *TwoFAChallenge, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, lookupError := service.Queries.GetUserByEmail(ctx, email)
	if lookupError != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if user.Banned {
		return nil, nil, ErrUserBanned
	}

	if compareError := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); compareError != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// Check email verification when SMTP is configured
	if service.smtpConfig != nil && !user.EmailVerified {
		return nil, nil, ErrEmailNotVerified
	}

	// Check if 2FA is enabled
	if user.TotpEnabled {
		twoFAToken, tokenError := service.generateTwoFAToken(user.ID)
		if tokenError != nil {
			return nil, nil, tokenError
		}
		return nil, &TwoFAChallenge{
			Requires2FA: true,
			TwoFAToken:  twoFAToken,
		}, nil
	}

	tokens, tokenError := service.issueTokens(ctx, user)
	if tokenError != nil {
		return nil, nil, tokenError
	}
	return tokens, nil, nil
}

// Verify2FA validates a TOTP code or recovery code and issues tokens.
func (service *AuthService) Verify2FA(ctx context.Context, twoFATokenString, code string) (*AuthTokens, error) {
	// Parse the 2FA pending token
	claims, parseError := service.parseToken(twoFATokenString)
	if parseError != nil {
		return nil, ErrInvalidToken
	}

	// Ensure it's a 2FA pending token
	pending2FA, _ := claims["pending_2fa"].(bool)
	if !pending2FA {
		return nil, ErrInvalidToken
	}

	userIDStr, _ := claims["sub"].(string)
	userID, parseUUIDError := uuid.Parse(userIDStr)
	if parseUUIDError != nil {
		return nil, ErrInvalidToken
	}

	user, lookupError := service.Queries.GetUserByID(ctx, userID)
	if lookupError != nil {
		return nil, ErrInvalidToken
	}

	if user.Banned {
		return nil, ErrUserBanned
	}

	if !user.TotpEnabled || !user.TotpSecret.Valid {
		return nil, ErrInvalidToken
	}

	// Try TOTP code first
	code = strings.TrimSpace(code)
	if totp.Validate(code, user.TotpSecret.String) {
		tokens, tokenError := service.issueTokens(ctx, user)
		if tokenError != nil {
			return nil, tokenError
		}
		return tokens, nil
	}

	// Try recovery code
	normalizedCode := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	for index, hashedCode := range user.RecoveryCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(normalizedCode)) == nil {
			// Remove used recovery code
			remainingCodes := append(user.RecoveryCodes[:index], user.RecoveryCodes[index+1:]...)
			if updateError := service.Queries.SetUserRecoveryCodes(ctx, db.SetUserRecoveryCodesParams{
				ID:            user.ID,
				RecoveryCodes: remainingCodes,
			}); updateError != nil {
				log.Printf("warning: failed to remove used recovery code: %v", updateError)
			}

			tokens, tokenError := service.issueTokens(ctx, user)
			if tokenError != nil {
				return nil, tokenError
			}
			return tokens, nil
		}
	}

	return nil, ErrInvalidTOTPCode
}

// RefreshTokens validates a refresh token and issues a new token pair (rotation).
func (service *AuthService) RefreshTokens(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	tokenHash := hashToken(refreshToken)

	storedToken, lookupError := service.Queries.GetRefreshTokenByHash(ctx, tokenHash)
	if lookupError != nil {
		return nil, ErrInvalidToken
	}

	// Delete the old refresh token (rotation)
	if deleteError := service.Queries.DeleteRefreshToken(ctx, tokenHash); deleteError != nil {
		log.Printf("warning: failed to delete old refresh token: %v", deleteError)
	}

	user, userError := service.Queries.GetUserByID(ctx, storedToken.UserID)
	if userError != nil {
		return nil, ErrInvalidToken
	}

	if user.Banned {
		return nil, ErrUserBanned
	}

	tokens, tokenError := service.issueTokens(ctx, user)
	if tokenError != nil {
		return nil, tokenError
	}
	return tokens, nil
}

// RevokeRefreshToken deletes a refresh token (logout).
func (service *AuthService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)
	return service.Queries.DeleteRefreshToken(ctx, tokenHash)
}

// RevokeAllUserTokens deletes all refresh tokens for a user (ban, password reset).
func (service *AuthService) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	return service.Queries.DeleteRefreshTokensByUserID(ctx, userID)
}

// Setup2FA generates a new TOTP secret for the user.
func (service *AuthService) Setup2FA(ctx context.Context, userID uuid.UUID) (*TOTPSetupInfo, error) {
	user, lookupError := service.Queries.GetUserByID(ctx, userID)
	if lookupError != nil {
		return nil, lookupError
	}

	if user.TotpEnabled {
		return nil, ErrTOTPAlreadyEnabled
	}

	key, generateError := totp.Generate(totp.GenerateOpts{
		Issuer:      service.instanceName,
		AccountName: user.Email,
	})
	if generateError != nil {
		return nil, fmt.Errorf("generating TOTP key: %w", generateError)
	}

	// Store the secret (not yet enabled)
	if setError := service.Queries.SetUserTOTPSecret(ctx, db.SetUserTOTPSecretParams{
		ID:            userID,
		TotpSecret:    sql.NullString{String: key.Secret(), Valid: true},
		TotpEnabled:   false,
		RecoveryCodes: nil,
	}); setError != nil {
		return nil, setError
	}

	return &TOTPSetupInfo{
		Secret:     key.Secret(),
		OTPAuthURL: key.URL(),
	}, nil
}

// Enable2FA verifies a TOTP code and enables 2FA, returning recovery codes.
func (service *AuthService) Enable2FA(ctx context.Context, userID uuid.UUID, code string) ([]string, error) {
	user, lookupError := service.Queries.GetUserByID(ctx, userID)
	if lookupError != nil {
		return nil, lookupError
	}

	if user.TotpEnabled {
		return nil, ErrTOTPAlreadyEnabled
	}

	if !user.TotpSecret.Valid {
		return nil, ErrInvalidInput
	}

	// Validate the code against the stored secret
	if !totp.Validate(code, user.TotpSecret.String) {
		return nil, ErrInvalidTOTPCode
	}

	// Generate recovery codes
	plaintextCodes := make([]string, recoveryCodeCount)
	hashedCodes := make([]string, recoveryCodeCount)
	for index := range recoveryCodeCount {
		plainCode := generateRecoveryCode()
		plaintextCodes[index] = formatRecoveryCode(plainCode)
		hash, hashError := bcrypt.GenerateFromPassword([]byte(plainCode), bcryptCost)
		if hashError != nil {
			return nil, fmt.Errorf("hashing recovery code: %w", hashError)
		}
		hashedCodes[index] = string(hash)
	}

	// Enable 2FA with recovery codes
	if setError := service.Queries.SetUserTOTPSecret(ctx, db.SetUserTOTPSecretParams{
		ID:            userID,
		TotpSecret:    user.TotpSecret,
		TotpEnabled:   true,
		RecoveryCodes: hashedCodes,
	}); setError != nil {
		return nil, setError
	}

	service.InvalidateUserCache(userID)
	return plaintextCodes, nil
}

// Disable2FA disables 2FA after verifying the user's password.
func (service *AuthService) Disable2FA(ctx context.Context, userID uuid.UUID, password string) error {
	user, lookupError := service.Queries.GetUserByID(ctx, userID)
	if lookupError != nil {
		return lookupError
	}

	if !user.TotpEnabled {
		return ErrTOTPNotEnabled
	}

	if compareError := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); compareError != nil {
		return ErrInvalidCredentials
	}

	if clearError := service.Queries.ClearUserTOTP(ctx, userID); clearError != nil {
		return clearError
	}

	service.InvalidateUserCache(userID)
	return nil
}

// ForgotPassword sends a password reset email if SMTP is configured.
func (service *AuthService) ForgotPassword(ctx context.Context, email string) error {
	if service.smtpConfig == nil {
		return ErrSMTPNotConfigured
	}

	email = strings.TrimSpace(strings.ToLower(email))
	user, lookupError := service.Queries.GetUserByEmail(ctx, email)
	if lookupError != nil {
		// Don't reveal whether the email exists
		return nil
	}

	resetToken, tokenError := generateRandomToken()
	if tokenError != nil {
		return tokenError
	}
	tokenHash := hashToken(resetToken)

	if setError := service.Queries.SetPasswordResetToken(ctx, db.SetPasswordResetTokenParams{
		ID:                     user.ID,
		PasswordResetToken:     sql.NullString{String: tokenHash, Valid: true},
		PasswordResetExpiresAt: sql.NullTime{Time: time.Now().Add(resetTokenLifetime), Valid: true},
	}); setError != nil {
		return setError
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", service.frontendURL, resetToken)
	emailBody := fmt.Sprintf(
		"<h2>Password Reset</h2><p>Click the link below to reset your password:</p><p><a href=\"%s\">Reset Password</a></p><p>This link expires in 1 hour. If you didn't request this, you can safely ignore this email.</p>",
		resetURL,
	)

	if sendError := service.sendEmail(email, "Reset your password", emailBody); sendError != nil {
		log.Printf("warning: failed to send password reset email: %v", sendError)
	}

	return nil
}

// ResetPassword validates a reset token and updates the password.
func (service *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}

	tokenHash := hashToken(token)
	user, lookupError := service.Queries.GetUserByPasswordResetToken(ctx, sql.NullString{String: tokenHash, Valid: true})
	if lookupError != nil {
		return ErrInvalidToken
	}

	passwordHash, hashError := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if hashError != nil {
		return fmt.Errorf("hashing password: %w", hashError)
	}

	if setError := service.Queries.SetUserPasswordHash(ctx, db.SetUserPasswordHashParams{
		ID:           user.ID,
		PasswordHash: string(passwordHash),
	}); setError != nil {
		return setError
	}

	// Clear the reset token
	if clearError := service.Queries.ClearPasswordResetToken(ctx, user.ID); clearError != nil {
		log.Printf("warning: failed to clear password reset token: %v", clearError)
	}

	// Revoke all refresh tokens (force re-login)
	if revokeError := service.Queries.DeleteRefreshTokensByUserID(ctx, user.ID); revokeError != nil {
		log.Printf("warning: failed to revoke refresh tokens after password reset: %v", revokeError)
	}

	service.InvalidateUserCache(user.ID)
	return nil
}

// ChangePassword validates the current password and updates to a new one.
func (service *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}

	user, lookupError := service.Queries.GetUserByID(ctx, userID)
	if lookupError != nil {
		return lookupError
	}

	if compareError := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); compareError != nil {
		return ErrInvalidCredentials
	}

	passwordHash, hashError := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if hashError != nil {
		return fmt.Errorf("hashing password: %w", hashError)
	}

	if setError := service.Queries.SetUserPasswordHash(ctx, db.SetUserPasswordHashParams{
		ID:           user.ID,
		PasswordHash: string(passwordHash),
	}); setError != nil {
		return setError
	}

	// Revoke all refresh tokens (force re-login on all devices)
	if revokeError := service.Queries.DeleteRefreshTokensByUserID(ctx, user.ID); revokeError != nil {
		log.Printf("warning: failed to revoke refresh tokens after password change: %v", revokeError)
	}

	service.InvalidateUserCache(user.ID)
	return nil
}

// VerifyEmail validates an email verification token.
func (service *AuthService) VerifyEmail(ctx context.Context, token string) error {
	tokenHash := hashToken(token)
	user, lookupError := service.Queries.GetUserByEmailVerifyToken(ctx, sql.NullString{String: tokenHash, Valid: true})
	if lookupError != nil {
		return ErrInvalidToken
	}

	if setError := service.Queries.SetEmailVerified(ctx, user.ID); setError != nil {
		return setError
	}

	service.InvalidateUserCache(user.ID)
	return nil
}

// ValidateAccessToken validates an HS256 JWT access token.
func (service *AuthService) ValidateAccessToken(tokenString string) (jwt.MapClaims, error) {
	token, parseError := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return service.jwtSecret, nil
	})
	if parseError != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Reject 2FA pending tokens from being used as access tokens
	if pending, exists := claims["pending_2fa"]; exists && pending == true {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// LookupUser retrieves a user by ID, using the in-memory cache.
func (service *AuthService) LookupUser(ctx context.Context, userID uuid.UUID) (db.User, error) {
	if cached, ok := service.userCache.Load(userID); ok {
		entry := cached.(cachedUser)
		if time.Since(entry.fetchedAt) < userCacheTTL {
			if entry.user.Banned {
				return db.User{}, ErrUserBanned
			}
			return entry.user, nil
		}
		service.userCache.Delete(userID)
	}

	user, lookupError := service.Queries.GetUserByID(ctx, userID)
	if lookupError != nil {
		return db.User{}, lookupError
	}

	if user.Banned {
		return db.User{}, ErrUserBanned
	}

	service.userCache.Store(userID, cachedUser{user: user, fetchedAt: time.Now()})
	return user, nil
}

// InvalidateUserCache removes a user from the in-memory cache.
func (service *AuthService) InvalidateUserCache(userID uuid.UUID) {
	service.userCache.Delete(userID)
}

func (service *AuthService) IsOpenRegistration() bool {
	return service.openRegistration
}

func (service *AuthService) SetOpenRegistration(open bool) {
	service.openRegistration = open
}

func (service *AuthService) GetInstanceName() string {
	return service.instanceName
}

func (service *AuthService) SetInstanceName(name string) {
	if name != "" {
		service.instanceName = name
	}
}

// ValidateInviteCode checks whether an invite code is valid.
func (service *AuthService) ValidateInviteCode(ctx context.Context, code string) bool {
	if code == "" {
		return false
	}
	inviteCode, lookupError := service.Queries.GetInviteCodeByCode(ctx, code)
	if lookupError != nil {
		return false
	}
	if inviteCode.ExpiresAt.Valid && inviteCode.ExpiresAt.Time.Before(time.Now()) {
		return false
	}
	if inviteCode.MaxUses.Valid && inviteCode.UseCount >= inviteCode.MaxUses.Int32 {
		return false
	}
	return true
}

// --- Internal helpers ---

// issueTokens generates a new access token and refresh token pair.
func (service *AuthService) issueTokens(ctx context.Context, user db.User) (*AuthTokens, error) {
	accessToken, accessError := service.generateAccessToken(user)
	if accessError != nil {
		return nil, accessError
	}

	refreshToken, refreshError := generateRandomToken()
	if refreshError != nil {
		return nil, refreshError
	}

	tokenHash := hashToken(refreshToken)
	if _, createError := service.Queries.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(refreshTokenLifetime),
	}); createError != nil {
		return nil, fmt.Errorf("storing refresh token: %w", createError)
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTokenLifetime.Seconds()),
	}, nil
}

// generateAccessToken creates a signed HS256 JWT.
func (service *AuthService) generateAccessToken(user db.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      user.ID.String(),
		"username": user.Username,
		"is_admin": user.IsAdmin,
		"iat":      now.Unix(),
		"exp":      now.Add(accessTokenLifetime).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(service.jwtSecret)
}

// generateTwoFAToken creates a short-lived JWT for 2FA pending state.
func (service *AuthService) generateTwoFAToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":         userID.String(),
		"pending_2fa": true,
		"iat":         now.Unix(),
		"exp":         now.Add(twoFATokenLifetime).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(service.jwtSecret)
}

// parseToken parses and validates an HS256 JWT without rejecting 2FA tokens.
func (service *AuthService) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, parseError := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return service.jwtSecret, nil
	})
	if parseError != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// sendEmail sends an HTML email via SMTP.
func (service *AuthService) sendEmail(to, subject, htmlBody string) error {
	if service.smtpConfig == nil {
		return ErrSMTPNotConfigured
	}

	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		service.smtpConfig.From, to, subject)
	message := []byte(headers + htmlBody)

	addr := fmt.Sprintf("%s:%d", service.smtpConfig.Host, service.smtpConfig.Port)
	auth := smtp.PlainAuth("", service.smtpConfig.Username, service.smtpConfig.Password, service.smtpConfig.Host)

	return smtp.SendMail(addr, auth, service.smtpConfig.From, []string{to}, message)
}

// generateRandomToken generates a cryptographically random URL-safe token.
func generateRandomToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, readError := rand.Read(tokenBytes); readError != nil {
		return "", fmt.Errorf("generating random token: %w", readError)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(tokenBytes), nil
}

// hashToken returns the hex-encoded SHA-256 hash of a token.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateRecoveryCode generates an 8-character alphanumeric recovery code.
func generateRecoveryCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1 to avoid confusion
	codeBytes := make([]byte, 8)
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	for index := range codeBytes {
		codeBytes[index] = charset[int(randomBytes[index])%len(charset)]
	}
	return string(codeBytes)
}

// formatRecoveryCode formats an 8-char code as XXXX-XXXX.
func formatRecoveryCode(code string) string {
	if len(code) == 8 {
		return code[:4] + "-" + code[4:]
	}
	return code
}

// isValidEmail performs a basic email format check.
func isValidEmail(email string) bool {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return strings.Contains(parts[1], ".")
}
