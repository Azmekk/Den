package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/service"
)

type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUsername contextKey = "username"
	ctxIsAdmin  contextKey = "is_admin"
)

// RequireAuth validates a Supabase JWT from the Authorization header,
// syncs/looks up the Den user, and populates request context with user info.
func RequireAuth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			header := request.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httputil.WriteError(writer, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}
			tokenString := strings.TrimPrefix(header, "Bearer ")

			claims, validationError := authSvc.ValidateSupabaseToken(tokenString)
			if validationError != nil {
				if errors.Is(validationError, service.ErrUserBanned) {
					httputil.WriteError(writer, http.StatusForbidden, "account is banned")
					return
				}
				httputil.WriteError(writer, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Look up or create the Den user from Supabase claims
			user, _, syncError := authSvc.SyncUser(request.Context(), claims)
			if syncError != nil {
				if errors.Is(syncError, service.ErrUserBanned) {
					httputil.WriteError(writer, http.StatusForbidden, "account is banned")
					return
				}
				if errors.Is(syncError, service.ErrInviteRequired) {
					httputil.WriteError(writer, http.StatusForbidden, "valid invite code required")
					return
				}
				httputil.WriteError(writer, http.StatusUnauthorized, "failed to resolve user")
				return
			}

			ctx := context.WithValue(request.Context(), ctxUserID, user.ID)
			ctx = context.WithValue(ctx, ctxUsername, user.Username)
			ctx = context.WithValue(ctx, ctxIsAdmin, user.IsAdmin)

			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !IsAdminFromContext(request.Context()) {
			httputil.WriteError(writer, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ctxUserID).(uuid.UUID)
	return id
}

func UsernameFromContext(ctx context.Context) string {
	username, _ := ctx.Value(ctxUsername).(string)
	return username
}

func IsAdminFromContext(ctx context.Context) bool {
	isAdmin, _ := ctx.Value(ctxIsAdmin).(bool)
	return isAdmin
}
