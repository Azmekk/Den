package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrRegistrationClosed = errors.New("registration is closed")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUserBanned         = errors.New("account is banned")
	ErrInviteRequired     = errors.New("valid invite code required")
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

var reservedUsernames = map[string]bool{
	"everyone": true,
	"here":     true,
	"channel":  true,
	"admin":    true,
}

// cachedUser holds a looked-up Den user with a TTL for the in-memory cache.
type cachedUser struct {
	user      db.User
	fetchedAt time.Time
}

const userCacheTTL = 2 * time.Minute

type AuthService struct {
	Queries          *db.Queries
	jwks             keyfunc.Keyfunc
	supabaseURL      string
	serviceRoleKey   string
	openRegistration bool
	instanceName     string

	// userCache maps supabase_id -> cached Den user to avoid DB lookups on every request.
	userCache sync.Map
}

func NewAuthService(queries *db.Queries, supabaseURL, serviceRoleKey string) *AuthService {
	jwksURL := strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"

	jwks, jwksError := keyfunc.NewDefault([]string{jwksURL})
	if jwksError != nil {
		log.Fatalf("failed to initialize JWKS from %s: %v", jwksURL, jwksError)
	}

	return &AuthService{
		Queries:        queries,
		jwks:           jwks,
		supabaseURL:    strings.TrimRight(supabaseURL, "/"),
		serviceRoleKey: serviceRoleKey,
		instanceName:   "Den",
	}
}

type UserInfo struct {
	ID            uuid.UUID `json:"id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name,omitempty"`
	IsAdmin       bool      `json:"is_admin"`
	NeedsUsername bool      `json:"needs_username"`
}

func UserInfoFromDB(user db.User) UserInfo {
	info := UserInfo{
		ID:            user.ID,
		Username:      user.Username,
		IsAdmin:       user.IsAdmin,
		NeedsUsername: user.NeedsUsername,
	}
	if user.DisplayName.Valid {
		info.DisplayName = user.DisplayName.String
	}
	return info
}

// ValidateSupabaseToken validates a Supabase JWT using JWKS (supports RS256 and other algorithms
// advertised by the Supabase JWKS endpoint).
func (service *AuthService) ValidateSupabaseToken(tokenString string) (jwt.MapClaims, error) {
	token, parseError := jwt.Parse(tokenString, service.jwks.Keyfunc)
	if parseError != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check banned_until claim — reject if set and in the future
	if bannedUntil, exists := claims["banned_until"]; exists && bannedUntil != nil {
		if bannedStr, ok := bannedUntil.(string); ok && bannedStr != "" {
			if bannedTime, parseErr := time.Parse(time.RFC3339, bannedStr); parseErr == nil {
				if bannedTime.After(time.Now()) {
					return nil, ErrUserBanned
				}
			}
		}
	}

	return claims, nil
}

// SyncUser looks up or creates a Den user from Supabase JWT claims.
// On first request from a new Supabase user, a local Den user is created.
// The first user to register becomes admin.
func (service *AuthService) SyncUser(ctx context.Context, claims jwt.MapClaims) (db.User, error) {
	supabaseID, _ := claims["sub"].(string)
	if supabaseID == "" {
		return db.User{}, ErrInvalidToken
	}

	// Check in-memory cache first
	if cached, ok := service.userCache.Load(supabaseID); ok {
		entry := cached.(cachedUser)
		if time.Since(entry.fetchedAt) < userCacheTTL {
			if entry.user.Banned {
				return db.User{}, ErrUserBanned
			}
			return entry.user, nil
		}
		service.userCache.Delete(supabaseID)
	}

	// Try DB lookup by supabase_id
	user, lookupError := service.Queries.GetUserBySupabaseID(ctx, sql.NullString{String: supabaseID, Valid: true})
	if lookupError == nil {
		if user.Banned {
			return db.User{}, ErrUserBanned
		}
		service.userCache.Store(supabaseID, cachedUser{user: user, fetchedAt: time.Now()})
		return user, nil
	}

	// User doesn't exist — create from Supabase claims
	email, _ := claims["email"].(string)

	// Check if the frontend sent an explicit username via user_metadata
	var explicitUsername string
	var displayName string
	var inviteCode string
	needsUsername := true

	if metadata, ok := claims["user_metadata"].(map[string]any); ok {
		if name, ok := metadata["username"].(string); ok && name != "" {
			if usernameRegex.MatchString(name) && len(name) <= 32 && !reservedUsernames[strings.ToLower(name)] {
				explicitUsername = name
				needsUsername = false
			}
		}
		// Extract display name from metadata
		if name, ok := metadata["display_name"].(string); ok && name != "" {
			displayName = name
		} else if name, ok := metadata["full_name"].(string); ok && name != "" {
			displayName = name
		} else if name, ok := metadata["name"].(string); ok && name != "" {
			displayName = name
		}
		// Extract invite code from metadata
		if code, ok := metadata["invite_code"].(string); ok {
			inviteCode = code
		}
	}

	// Enforce invite code when registration is closed (skip for first user)
	userCount, countError := service.Queries.CountUsers(ctx)
	if countError != nil {
		return db.User{}, countError
	}
	isFirstUser := userCount == 0

	var validatedInviteCode *db.InviteCode
	if !isFirstUser && !service.openRegistration {
		if inviteCode == "" {
			// No invite code provided — clean up the Supabase account
			if deleteError := service.SupabaseDeleteUser(supabaseID); deleteError != nil {
				log.Printf("warning: failed to delete supabase user after invite rejection: %v", deleteError)
			}
			return db.User{}, ErrInviteRequired
		}
		code, lookupError := service.Queries.GetInviteCodeByCode(ctx, inviteCode)
		if lookupError != nil {
			if deleteError := service.SupabaseDeleteUser(supabaseID); deleteError != nil {
				log.Printf("warning: failed to delete supabase user after invite rejection: %v", deleteError)
			}
			return db.User{}, ErrInviteRequired
		}
		// Validate expiry
		if code.ExpiresAt.Valid && code.ExpiresAt.Time.Before(time.Now()) {
			if deleteError := service.SupabaseDeleteUser(supabaseID); deleteError != nil {
				log.Printf("warning: failed to delete supabase user after invite rejection: %v", deleteError)
			}
			return db.User{}, ErrInviteRequired
		}
		// Validate use count
		if code.MaxUses.Valid && code.UseCount >= code.MaxUses.Int32 {
			if deleteError := service.SupabaseDeleteUser(supabaseID); deleteError != nil {
				log.Printf("warning: failed to delete supabase user after invite rejection: %v", deleteError)
			}
			return db.User{}, ErrInviteRequired
		}
		validatedInviteCode = &code
	}

	var username string
	if explicitUsername != "" {
		username = explicitUsername
	} else {
		username = deriveUsername(email, supabaseID)
	}

	user, createError := service.Queries.CreateUser(ctx, db.CreateUserParams{
		Username:      username,
		DisplayName:   sql.NullString{String: displayName, Valid: displayName != ""},
		IsAdmin:       isFirstUser,
		SupabaseID:    sql.NullString{String: supabaseID, Valid: true},
		NeedsUsername: needsUsername,
	})
	if createError != nil {
		if isUniqueViolation(createError) {
			// Username collision — append random suffix and retry
			user, createError = service.Queries.CreateUser(ctx, db.CreateUserParams{
				Username:      username + "-" + uuid.New().String()[:8],
				DisplayName:   sql.NullString{String: displayName, Valid: displayName != ""},
				IsAdmin:       isFirstUser,
				SupabaseID:    sql.NullString{String: supabaseID, Valid: true},
				NeedsUsername: needsUsername,
			})
			if createError != nil {
				return db.User{}, createError
			}
		} else {
			return db.User{}, createError
		}
	}

	// Increment invite code use count after successful creation
	if validatedInviteCode != nil {
		if incrementError := service.Queries.IncrementInviteCodeUseCount(ctx, validatedInviteCode.ID); incrementError != nil {
			log.Printf("warning: failed to increment invite code use count: %v", incrementError)
		}
	}

	service.userCache.Store(supabaseID, cachedUser{user: user, fetchedAt: time.Now()})
	return user, nil
}

// SetUsername sets the username for a user who needs one (e.g. OAuth users).
func (service *AuthService) SetUsername(ctx context.Context, userID uuid.UUID, username string) (db.User, error) {
	if !usernameRegex.MatchString(username) || len(username) > 32 || len(username) == 0 {
		return db.User{}, ErrInvalidInput
	}
	if reservedUsernames[strings.ToLower(username)] {
		return db.User{}, ErrInvalidInput
	}

	user, updateError := service.Queries.SetUserUsername(ctx, db.SetUserUsernameParams{
		ID:       userID,
		Username: username,
	})
	if updateError != nil {
		if isUniqueViolation(updateError) {
			return db.User{}, ErrUsernameTaken
		}
		return db.User{}, updateError
	}

	// Invalidate cache so subsequent requests pick up the new username
	if user.SupabaseID.Valid {
		service.userCache.Delete(user.SupabaseID.String)
	}

	return user, nil
}

// InvalidateUserCache removes a user from the in-memory cache, forcing a DB lookup on next request.
func (service *AuthService) InvalidateUserCache(supabaseID string) {
	service.userCache.Delete(supabaseID)
}

// SupabaseBanUser bans a user on Supabase by setting a very long ban duration.
func (service *AuthService) SupabaseBanUser(supabaseID string) error {
	return service.supabaseAdminUpdateUser(supabaseID, map[string]any{
		"ban_duration": "876000h",
	})
}

// SupabaseUnbanUser removes the ban on a Supabase user.
func (service *AuthService) SupabaseUnbanUser(supabaseID string) error {
	return service.supabaseAdminUpdateUser(supabaseID, map[string]any{
		"ban_duration": "none",
	})
}

// SupabaseDeleteUser deletes a user from Supabase.
func (service *AuthService) SupabaseDeleteUser(supabaseID string) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", service.supabaseURL, supabaseID)
	request, requestError := http.NewRequest(http.MethodDelete, url, nil)
	if requestError != nil {
		return fmt.Errorf("creating delete request: %w", requestError)
	}
	request.Header.Set("Authorization", "Bearer "+service.serviceRoleKey)
	request.Header.Set("apikey", service.serviceRoleKey)

	response, doError := http.DefaultClient.Do(request)
	if doError != nil {
		return fmt.Errorf("supabase delete user request failed: %w", doError)
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("supabase delete user returned %d: %s", response.StatusCode, string(body))
	}
	return nil
}

// supabaseAdminUpdateUser sends a PUT request to update a Supabase user via the admin API.
func (service *AuthService) supabaseAdminUpdateUser(supabaseID string, body map[string]any) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", service.supabaseURL, supabaseID)
	jsonBody, marshalError := json.Marshal(body)
	if marshalError != nil {
		return fmt.Errorf("marshaling request body: %w", marshalError)
	}

	request, requestError := http.NewRequest(http.MethodPut, url, bytes.NewReader(jsonBody))
	if requestError != nil {
		return fmt.Errorf("creating request: %w", requestError)
	}
	request.Header.Set("Authorization", "Bearer "+service.serviceRoleKey)
	request.Header.Set("apikey", service.serviceRoleKey)
	request.Header.Set("Content-Type", "application/json")

	response, doError := http.DefaultClient.Do(request)
	if doError != nil {
		return fmt.Errorf("supabase admin request failed: %w", doError)
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(response.Body)
		return fmt.Errorf("supabase admin returned %d: %s", response.StatusCode, string(responseBody))
	}
	return nil
}

// deriveUsername creates a username from the email or Supabase ID.
func deriveUsername(email, supabaseID string) string {
	if email != "" {
		parts := strings.SplitN(email, "@", 2)
		candidate := strings.ToLower(parts[0])
		// Strip invalid characters
		candidate = usernameRegex.FindString(candidate)
		if candidate != "" && len(candidate) <= 32 && !reservedUsernames[candidate] {
			return candidate
		}
	}
	// Fallback: use first 8 chars of supabase ID
	if len(supabaseID) > 8 {
		return "user-" + supabaseID[:8]
	}
	return "user-" + supabaseID
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

// ValidateInviteCode checks whether an invite code is valid (exists, not expired, under max uses).
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
