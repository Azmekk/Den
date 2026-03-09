package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/ws"
)

type UserHandler struct {
	svc *service.UserService
	hub *ws.Hub
}

func NewUserHandler(svc *service.UserService, hub *ws.Hub) *UserHandler {
	return &UserHandler{svc: svc, hub: hub}
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
