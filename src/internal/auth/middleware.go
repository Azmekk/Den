package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUsername  contextKey = "username"
	ctxIsAdmin  contextKey = "is_admin"
)

func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}
		tokenString := strings.TrimPrefix(header, "Bearer ")

		claims, err := s.ValidateAccessToken(tokenString)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		sub, _ := claims["sub"].(string)
		userID, err := uuid.Parse(sub)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
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

func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return s.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func UserIDFromContext(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ctxUserID).(uuid.UUID)
	return id
}

func IsAdminFromContext(ctx context.Context) bool {
	isAdmin, _ := ctx.Value(ctxIsAdmin).(bool)
	return isAdmin
}
