package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/martinmckenna/den/internal/httputil"
	"github.com/martinmckenna/den/internal/service"
)

type contextKey string

const (
	ctxUserID  contextKey = "user_id"
	ctxUsername contextKey = "username"
	ctxIsAdmin contextKey = "is_admin"
)

func RequireAuth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httputil.WriteError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}
			tokenString := strings.TrimPrefix(header, "Bearer ")

			claims, err := authSvc.ValidateAccessToken(tokenString)
			if err != nil {
				httputil.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			sub, _ := claims["sub"].(string)
			userID, err := uuid.Parse(sub)
			if err != nil {
				httputil.WriteError(w, http.StatusUnauthorized, "invalid token claims")
				return
			}

			username, _ := claims["username"].(string)
			isAdmin, _ := claims["is_admin"].(bool)

			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			ctx = context.WithValue(ctx, ctxUsername, username)
			ctx = context.WithValue(ctx, ctxIsAdmin, isAdmin)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			httputil.WriteError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
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
