package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/ws"
)

type UserHandler struct {
	svc      *service.UserService
	mediaSvc *service.MediaService
	hub      *ws.Hub
}

func NewUserHandler(svc *service.UserService, mediaSvc *service.MediaService, hub *ws.Hub) *UserHandler {
	return &UserHandler{svc: svc, mediaSvc: mediaSvc, hub: hub}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}

type updateDisplayNameRequest struct {
	DisplayName string `json:"display_name"`
}

func (h *UserHandler) UpdateDisplayName(w http.ResponseWriter, r *http.Request) {
	var req updateDisplayNameRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.svc.UpdateDisplayName(r.Context(), userID, req.DisplayName)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Broadcast user_updated to all clients
	envelope, _ := json.Marshal(map[string]any{
		"type":         "user_updated",
		"id":           userID,
		"display_name": user.DisplayName,
	})
	h.hub.BroadcastGlobal(envelope)

	httputil.WriteJSON(w, http.StatusOK, user)
}

type updateColorRequest struct {
	Color string `json:"color"`
}

func (h *UserHandler) UpdateColor(w http.ResponseWriter, r *http.Request) {
	var req updateColorRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.svc.UpdateColor(r.Context(), userID, req.Color)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Broadcast user_updated to all clients
	envelope, _ := json.Marshal(map[string]any{
		"type":  "user_updated",
		"id":    userID,
		"color": user.Color,
	})
	h.hub.BroadcastGlobal(envelope)

	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	if h.mediaSvc == nil || !h.mediaSvc.IsConfigured() {
		httputil.WriteError(w, http.StatusNotImplemented, "uploads not configured")
		return
	}

	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "missing avatar file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.mediaSvc.UpdateAvatar(r.Context(), userID, data, h.svc.Queries())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaTooLarge):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrMediaBadFormat):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	envelope, _ := json.Marshal(map[string]any{
		"type":       "user_updated",
		"id":         userID,
		"avatar_url": user.AvatarURL,
	})
	h.hub.BroadcastGlobal(envelope)

	httputil.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	avatarURL, err := h.svc.GetAvatarURL(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusNotFound, "no avatar")
		return
	}

	http.Redirect(w, r, avatarURL, http.StatusFound)
}
