package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type AdminHandler struct {
	svc *service.AdminService
}

func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, users)
}

func (h *AdminHandler) SetAdmin(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.SetAdmin(r.Context(), callerID, targetID, req.IsAdmin); err != nil {
		if errors.Is(err, service.ErrSelfDemotion) {
			httputil.WriteError(w, http.StatusBadRequest, "cannot remove your own admin status")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "admin status updated"})
}

func (h *AdminHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	tempPassword, err := h.svc.ResetPassword(r.Context(), targetID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"temp_password": tempPassword})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if err := h.svc.DeleteUser(r.Context(), callerID, targetID); err != nil {
		if errors.Is(err, service.ErrSelfDeletion) {
			httputil.WriteError(w, http.StatusBadRequest, "cannot delete your own account")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) CleanupMessages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count int32 `json:"count"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil || req.Count <= 0 {
		httputil.WriteError(w, http.StatusBadRequest, "count must be a positive integer")
		return
	}

	if err := h.svc.DeleteOldestMessages(r.Context(), req.Count); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "messages deleted"})
}

func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, h.svc.GetSettings())
}

func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OpenRegistration *bool   `json:"open_registration"`
		InstanceName     *string `json:"instance_name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.svc.UpdateSettings(req.OpenRegistration, req.InstanceName)
	httputil.WriteJSON(w, http.StatusOK, h.svc.GetSettings())
}
