package handler

import (
	"errors"
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Me returns the current user's profile from the Den database.
func (handler *AuthHandler) Me(writer http.ResponseWriter, request *http.Request) {
	userID := middleware.UserIDFromContext(request.Context())
	user, lookupError := handler.authService.Queries.GetUserByID(request.Context(), userID)
	if lookupError != nil {
		httputil.WriteInternalError(writer, "internal error", lookupError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, service.UserInfoFromDB(user))
}

// ValidateInviteCode checks if an invite code is valid (public, no auth required).
func (handler *AuthHandler) ValidateInviteCode(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	valid := handler.authService.ValidateInviteCode(request.Context(), body.Code)
	httputil.WriteJSON(writer, http.StatusOK, map[string]bool{"valid": valid})
}

// SetUsername allows a user to set their username (typically after OAuth registration).
func (handler *AuthHandler) SetUsername(writer http.ResponseWriter, request *http.Request) {
	userID := middleware.UserIDFromContext(request.Context())

	var body struct {
		Username string `json:"username"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify user actually needs a username
	currentUser, lookupError := handler.authService.Queries.GetUserByID(request.Context(), userID)
	if lookupError != nil {
		httputil.WriteInternalError(writer, "internal error", lookupError)
		return
	}
	if !currentUser.NeedsUsername {
		httputil.WriteError(writer, http.StatusForbidden, "username already set")
		return
	}

	user, setError := handler.authService.SetUsername(request.Context(), userID, body.Username)
	if setError != nil {
		if errors.Is(setError, service.ErrUsernameTaken) {
			httputil.WriteError(writer, http.StatusConflict, "username already taken")
			return
		}
		if errors.Is(setError, service.ErrInvalidInput) {
			httputil.WriteError(writer, http.StatusBadRequest, "invalid username: must be 1-32 characters, alphanumeric with hyphens and underscores only")
			return
		}
		httputil.WriteInternalError(writer, "internal error", setError)
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, service.UserInfoFromDB(user))
}
