package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrRegistrationClosed = errors.New("registration is closed")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

type AuthService struct {
	Queries          *db.Queries
	jwtSecret        []byte
	openRegistration bool
	instanceName     string
}

func NewAuthService(queries *db.Queries, jwtSecret string, openRegistration bool) *AuthService {
	return &AuthService{
		Queries:          queries,
		jwtSecret:        []byte(jwtSecret),
		openRegistration: openRegistration,
		instanceName:     "Den",
	}
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type UserInfo struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	IsAdmin     bool      `json:"is_admin"`
}

func UserInfoFromDB(u db.User) UserInfo {
	info := UserInfo{
		ID:       u.ID,
		Username: u.Username,
		IsAdmin:  u.IsAdmin,
	}
	if u.DisplayName.Valid {
		info.DisplayName = u.DisplayName.String
	}
	return info
}

func (s *AuthService) Register(ctx context.Context, username, password, displayName string) (UserInfo, TokenPair, error) {
	if username == "" || password == "" {
		return UserInfo{}, TokenPair{}, ErrInvalidInput
	}
	if len(password) < 8 {
		return UserInfo{}, TokenPair{}, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	if len(username) > 32 {
		return UserInfo{}, TokenPair{}, fmt.Errorf("%w: username too long", ErrInvalidInput)
	}

	count, err := s.Queries.CountUsers(ctx)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}

	isFirstUser := count == 0
	if !isFirstUser && !s.openRegistration {
		return UserInfo{}, TokenPair{}, ErrRegistrationClosed
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}

	user, err := s.Queries.CreateUser(ctx, db.CreateUserParams{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  sql.NullString{String: displayName, Valid: displayName != ""},
		IsAdmin:      isFirstUser,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return UserInfo{}, TokenPair{}, ErrUsernameTaken
		}
		return UserInfo{}, TokenPair{}, err
	}

	tokens, err := s.IssueTokens(ctx, user)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}

	return UserInfoFromDB(user), tokens, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (UserInfo, TokenPair, error) {
	user, err := s.Queries.GetUserByUsername(ctx, username)
	if err != nil {
		return UserInfo{}, TokenPair{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return UserInfo{}, TokenPair{}, ErrInvalidCredentials
	}

	tokens, err := s.IssueTokens(ctx, user)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}

	return UserInfoFromDB(user), tokens, nil
}

func (s *AuthService) IssueTokens(ctx context.Context, user db.User) (TokenPair, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      user.ID.String(),
		"username": user.Username,
		"is_admin": user.IsAdmin,
		"exp":      now.Add(5 * time.Minute).Unix(),
		"iat":      now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return TokenPair{}, err
	}

	rawRefresh := make([]byte, 32)
	if _, err := rand.Read(rawRefresh); err != nil {
		return TokenPair{}, err
	}
	refreshToken := hex.EncodeToString(rawRefresh)
	hash := sha256Hash(refreshToken)

	_, err = s.Queries.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
	})
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) RefreshTokens(ctx context.Context, rawRefreshToken string) (UserInfo, TokenPair, error) {
	if rawRefreshToken == "" {
		return UserInfo{}, TokenPair{}, ErrInvalidToken
	}

	hash := sha256Hash(rawRefreshToken)
	stored, err := s.Queries.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return UserInfo{}, TokenPair{}, ErrInvalidToken
	}

	if time.Now().After(stored.ExpiresAt) {
		s.Queries.DeleteRefreshToken(ctx, stored.ID)
		return UserInfo{}, TokenPair{}, ErrInvalidToken
	}

	s.Queries.DeleteRefreshToken(ctx, stored.ID)

	user, err := s.Queries.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}

	tokens, err := s.IssueTokens(ctx, user)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}

	return UserInfoFromDB(user), tokens, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	if newPassword == "" || len(newPassword) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}

	user, err := s.Queries.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.Queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: string(hash),
	}); err != nil {
		return err
	}

	return s.Queries.DeleteRefreshTokensByUser(ctx, userID)
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	hash := sha256Hash(rawRefreshToken)
	stored, err := s.Queries.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil
	}
	return s.Queries.DeleteRefreshToken(ctx, stored.ID)
}

func (s *AuthService) ValidateAccessToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (s *AuthService) IsOpenRegistration() bool {
	return s.openRegistration
}

func (s *AuthService) SetOpenRegistration(open bool) {
	s.openRegistration = open
}

func (s *AuthService) GetInstanceName() string {
	return s.instanceName
}

func (s *AuthService) SetInstanceName(name string) {
	if name != "" {
		s.instanceName = name
	}
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
