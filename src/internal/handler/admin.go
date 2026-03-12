package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type AdminHandler struct {
	svc      *service.AdminService
	mediaSvc *service.MediaService
}

func NewAdminHandler(svc *service.AdminService, mediaSvc *service.MediaService) *AdminHandler {
	return &AdminHandler{svc: svc, mediaSvc: mediaSvc}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
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
		httputil.WriteInternalError(w, "internal error", err)
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
		httputil.WriteInternalError(w, "internal error", err)
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
		httputil.WriteInternalError(w, "internal error", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
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
		httputil.WriteInternalError(w, "internal error", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "messages deleted"})
}

func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, h.svc.GetSettings())
}

func parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 50
	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}
	return
}

func (h *AdminHandler) ListMedia(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	media, err := h.svc.ListMedia(r.Context(), page, pageSize)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, media)
}

func (h *AdminHandler) ListDeletedMedia(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	media, err := h.svc.ListDeletedMedia(r.Context(), page, pageSize)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, media)
}

func (h *AdminHandler) GetMediaStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetMediaStats(r.Context())
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid media id")
		return
	}
	if err := h.mediaSvc.DeleteMediaAdmin(r.Context(), id); err != nil {
		httputil.WriteInternalError(w, "failed to delete media", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "media deleted"})
}

func (h *AdminHandler) BulkDeleteMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil || len(req.IDs) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "ids must be a non-empty array")
		return
	}
	deleted := 0
	for _, id := range req.IDs {
		if err := h.mediaSvc.DeleteMediaAdmin(r.Context(), id); err == nil {
			deleted++
		}
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}

func (h *AdminHandler) ListInviteCodes(w http.ResponseWriter, r *http.Request) {
	codes, err := h.svc.ListInviteCodes(r.Context())
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, codes)
}

func (h *AdminHandler) CreateInviteCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxUses      *int32 `json:"max_uses"`
		ExpiresInHours *int `json:"expires_in_hours"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresInHours != nil && *req.ExpiresInHours > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresInHours) * time.Hour)
		expiresAt = &t
	}

	callerID := middleware.UserIDFromContext(r.Context())
	code, err := h.svc.CreateInviteCode(r.Context(), callerID, req.MaxUses, expiresAt)
	if err != nil {
		httputil.WriteInternalError(w, "failed to create invite code", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, code)
}

func (h *AdminHandler) DeleteInviteCode(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid invite code id")
		return
	}
	if err := h.svc.DeleteInviteCode(r.Context(), id); err != nil {
		httputil.WriteInternalError(w, "failed to delete invite code", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "invite code deleted"})
}

func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OpenRegistration *bool   `json:"open_registration"`
		InstanceName     *string `json:"instance_name"`
		MaxMessages      *int64  `json:"max_messages"`
		MaxMessageChars  *int    `json:"max_message_chars"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxMessages != nil && *req.MaxMessages < 0 {
		httputil.WriteError(w, http.StatusBadRequest, "max_messages must be >= 0")
		return
	}
	if req.MaxMessageChars != nil && (*req.MaxMessageChars < 1 || *req.MaxMessageChars > 10000) {
		httputil.WriteError(w, http.StatusBadRequest, "max_message_chars must be between 1 and 10000")
		return
	}

	if err := h.svc.UpdateSettings(r.Context(), req.OpenRegistration, req.InstanceName, req.MaxMessages, req.MaxMessageChars); err != nil {
		httputil.WriteInternalError(w, "failed to update settings", err)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.svc.GetSettings())
}
