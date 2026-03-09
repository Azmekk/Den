package handler

import (
	"errors"
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type registerRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type authResponse struct {
	AccessToken string           `json:"access_token"`
	User        service.UserInfo `json:"user"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := h.svc.Register(r.Context(), req.Username, req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrUsernameTaken):
			httputil.WriteError(w, http.StatusConflict, "username already taken")
		case errors.Is(err, service.ErrRegistrationClosed):
			httputil.WriteError(w, http.StatusForbidden, "registration is closed")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	httputil.SetRefreshTokenCookie(w, tokens.RefreshToken)
	httputil.WriteJSON(w, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		} else {
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	httputil.SetRefreshTokenCookie(w, tokens.RefreshToken)
	httputil.WriteJSON(w, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	user, tokens, err := h.svc.RefreshTokens(r.Context(), cookie.Value)
	if err != nil {
		httputil.ClearRefreshTokenCookie(w)
		httputil.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	httputil.SetRefreshTokenCookie(w, tokens.RefreshToken)
	httputil.WriteJSON(w, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		h.svc.Logout(r.Context(), cookie.Value)
	}

	httputil.ClearRefreshTokenCookie(w)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.svc.Queries.GetUserByID(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, service.UserInfoFromDB(user))
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	err := h.svc.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrInvalidCredentials):
			httputil.WriteError(w, http.StatusUnauthorized, "incorrect old password")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}
