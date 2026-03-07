package auth

import (
	"errors"
	"net/http"
)

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
	AccessToken string   `json:"access_token"`
	User        UserInfo `json:"user"`
}

func (s *Service) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := s.Register(r.Context(), req.Username, req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrUsernameTaken):
			writeError(w, http.StatusConflict, "username already taken")
		case errors.Is(err, ErrRegistrationClosed):
			writeError(w, http.StatusForbidden, "registration is closed")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	setRefreshTokenCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

func (s *Service) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := s.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
		} else {
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	setRefreshTokenCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

func (s *Service) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	user, tokens, err := s.RefreshTokens(r.Context(), cookie.Value)
	if err != nil {
		clearRefreshTokenCookie(w)
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	setRefreshTokenCookie(w, tokens.RefreshToken)
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

func (s *Service) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		s.Logout(r.Context(), cookie.Value)
	}

	clearRefreshTokenCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (s *Service) MeHandler(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := s.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, userInfoFromDB(user))
}

func (s *Service) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := UserIDFromContext(r.Context())
	err := s.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "incorrect old password")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}
