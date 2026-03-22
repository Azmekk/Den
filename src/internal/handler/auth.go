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

// Me returns the current user's profile.
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

// Register creates a new user account.
func (handler *AuthHandler) Register(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Email      string `json:"email"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		InviteCode string `json:"invite_code"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, emailVerificationRequired, registerError := handler.authService.Register(
		request.Context(), body.Email, body.Username, body.Password, body.InviteCode,
	)
	if registerError != nil {
		switch {
		case errors.Is(registerError, service.ErrInvalidInput):
			httputil.WriteError(writer, http.StatusBadRequest, "invalid input: username must be 1-32 characters (a-z, 0-9, ., -, _) and email must be valid")
		case errors.Is(registerError, service.ErrWeakPassword):
			httputil.WriteError(writer, http.StatusBadRequest, "password must be at least 8 characters")
		case errors.Is(registerError, service.ErrEmailTaken):
			httputil.WriteError(writer, http.StatusConflict, "email already taken")
		case errors.Is(registerError, service.ErrUsernameTaken):
			httputil.WriteError(writer, http.StatusConflict, "username already taken")
		case errors.Is(registerError, service.ErrInviteRequired):
			httputil.WriteError(writer, http.StatusForbidden, "valid invite code required")
		default:
			httputil.WriteInternalError(writer, "registration failed", registerError)
		}
		return
	}

	if emailVerificationRequired {
		httputil.WriteJSON(writer, http.StatusCreated, map[string]bool{"email_verification_required": true})
		return
	}

	httputil.WriteJSON(writer, http.StatusCreated, tokens)
}

// Login authenticates a user with email and password.
func (handler *AuthHandler) Login(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, twoFAChallenge, loginError := handler.authService.Login(request.Context(), body.Email, body.Password)
	if loginError != nil {
		switch {
		case errors.Is(loginError, service.ErrInvalidCredentials):
			httputil.WriteError(writer, http.StatusUnauthorized, "invalid email or password")
		case errors.Is(loginError, service.ErrUserBanned):
			httputil.WriteError(writer, http.StatusForbidden, "account is banned")
		case errors.Is(loginError, service.ErrEmailNotVerified):
			httputil.WriteError(writer, http.StatusForbidden, "email not verified")
		default:
			httputil.WriteInternalError(writer, "login failed", loginError)
		}
		return
	}

	if twoFAChallenge != nil {
		httputil.WriteJSON(writer, http.StatusOK, twoFAChallenge)
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, tokens)
}

// Verify2FA validates a TOTP or recovery code after login.
func (handler *AuthHandler) Verify2FA(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, verifyError := handler.authService.Verify2FA(request.Context(), body.Token, body.Code)
	if verifyError != nil {
		switch {
		case errors.Is(verifyError, service.ErrInvalidToken):
			httputil.WriteError(writer, http.StatusUnauthorized, "invalid or expired 2FA token")
		case errors.Is(verifyError, service.ErrInvalidTOTPCode):
			httputil.WriteError(writer, http.StatusUnauthorized, "invalid 2FA code")
		case errors.Is(verifyError, service.ErrUserBanned):
			httputil.WriteError(writer, http.StatusForbidden, "account is banned")
		default:
			httputil.WriteInternalError(writer, "2FA verification failed", verifyError)
		}
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, tokens)
}

// RefreshToken issues a new token pair using a refresh token.
func (handler *AuthHandler) RefreshToken(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, refreshError := handler.authService.RefreshTokens(request.Context(), body.RefreshToken)
	if refreshError != nil {
		if errors.Is(refreshError, service.ErrInvalidToken) || errors.Is(refreshError, service.ErrUserBanned) {
			httputil.WriteError(writer, http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		httputil.WriteInternalError(writer, "token refresh failed", refreshError)
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, tokens)
}

// Logout revokes the provided refresh token.
func (handler *AuthHandler) Logout(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	// Best-effort revocation — don't fail if token doesn't exist
	_ = handler.authService.RevokeRefreshToken(request.Context(), body.RefreshToken)
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "logged out"})
}

// ForgotPassword sends a password reset email.
func (handler *AuthHandler) ForgotPassword(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	forgotError := handler.authService.ForgotPassword(request.Context(), body.Email)
	if forgotError != nil {
		if errors.Is(forgotError, service.ErrSMTPNotConfigured) {
			httputil.WriteError(writer, http.StatusServiceUnavailable, "password reset is not available")
			return
		}
		httputil.WriteInternalError(writer, "failed to process request", forgotError)
		return
	}

	// Always return success to avoid leaking whether email exists
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "if that email exists, a reset link has been sent"})
}

// ResetPassword validates a reset token and updates the password.
func (handler *AuthHandler) ResetPassword(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	resetError := handler.authService.ResetPassword(request.Context(), body.Token, body.NewPassword)
	if resetError != nil {
		switch {
		case errors.Is(resetError, service.ErrInvalidToken):
			httputil.WriteError(writer, http.StatusBadRequest, "invalid or expired reset token")
		case errors.Is(resetError, service.ErrWeakPassword):
			httputil.WriteError(writer, http.StatusBadRequest, "password must be at least 8 characters")
		default:
			httputil.WriteInternalError(writer, "password reset failed", resetError)
		}
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "password reset successful"})
}

// VerifyEmail validates an email verification token.
func (handler *AuthHandler) VerifyEmail(writer http.ResponseWriter, request *http.Request) {
	token := request.URL.Query().Get("token")
	if token == "" {
		httputil.WriteError(writer, http.StatusBadRequest, "missing token")
		return
	}

	verifyError := handler.authService.VerifyEmail(request.Context(), token)
	if verifyError != nil {
		if errors.Is(verifyError, service.ErrInvalidToken) {
			httputil.WriteError(writer, http.StatusBadRequest, "invalid or expired verification token")
			return
		}
		httputil.WriteInternalError(writer, "email verification failed", verifyError)
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "email verified"})
}

// Setup2FA generates a TOTP secret for the authenticated user.
func (handler *AuthHandler) Setup2FA(writer http.ResponseWriter, request *http.Request) {
	userID := middleware.UserIDFromContext(request.Context())

	setupInfo, setupError := handler.authService.Setup2FA(request.Context(), userID)
	if setupError != nil {
		if errors.Is(setupError, service.ErrTOTPAlreadyEnabled) {
			httputil.WriteError(writer, http.StatusConflict, "2FA is already enabled")
			return
		}
		httputil.WriteInternalError(writer, "2FA setup failed", setupError)
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, setupInfo)
}

// Enable2FA verifies a TOTP code and enables 2FA.
func (handler *AuthHandler) Enable2FA(writer http.ResponseWriter, request *http.Request) {
	userID := middleware.UserIDFromContext(request.Context())

	var body struct {
		Code string `json:"code"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	recoveryCodes, enableError := handler.authService.Enable2FA(request.Context(), userID, body.Code)
	if enableError != nil {
		switch {
		case errors.Is(enableError, service.ErrTOTPAlreadyEnabled):
			httputil.WriteError(writer, http.StatusConflict, "2FA is already enabled")
		case errors.Is(enableError, service.ErrInvalidTOTPCode):
			httputil.WriteError(writer, http.StatusBadRequest, "invalid verification code")
		case errors.Is(enableError, service.ErrInvalidInput):
			httputil.WriteError(writer, http.StatusBadRequest, "2FA setup not started")
		default:
			httputil.WriteInternalError(writer, "2FA enable failed", enableError)
		}
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, map[string]any{"recovery_codes": recoveryCodes})
}

// Disable2FA disables 2FA after password verification.
func (handler *AuthHandler) Disable2FA(writer http.ResponseWriter, request *http.Request) {
	userID := middleware.UserIDFromContext(request.Context())

	var body struct {
		Password string `json:"password"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	disableError := handler.authService.Disable2FA(request.Context(), userID, body.Password)
	if disableError != nil {
		switch {
		case errors.Is(disableError, service.ErrTOTPNotEnabled):
			httputil.WriteError(writer, http.StatusBadRequest, "2FA is not enabled")
		case errors.Is(disableError, service.ErrInvalidCredentials):
			httputil.WriteError(writer, http.StatusUnauthorized, "invalid password")
		default:
			httputil.WriteInternalError(writer, "2FA disable failed", disableError)
		}
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "2FA disabled"})
}
