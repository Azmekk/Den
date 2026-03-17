package handler

import (
	"database/sql"
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

func (handler *AdminHandler) ListUsers(writer http.ResponseWriter, request *http.Request) {
	users, fetchError := handler.svc.ListUsers(request.Context())
	if fetchError != nil {
		httputil.WriteInternalError(writer, "internal error", fetchError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, users)
}

func (handler *AdminHandler) SetAdmin(writer http.ResponseWriter, request *http.Request) {
	targetID, parseError := uuid.Parse(chi.URLParam(request, "id"))
	if parseError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid user id")
		return
	}

	var body struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := httputil.DecodeJSON(request, &body); err != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(request.Context())
	if err := handler.svc.SetAdmin(request.Context(), callerID, targetID, body.IsAdmin); err != nil {
		if errors.Is(err, service.ErrSelfDemotion) {
			httputil.WriteError(writer, http.StatusBadRequest, "cannot remove your own admin status")
			return
		}
		httputil.WriteInternalError(writer, "internal error", err)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "admin status updated"})
}

func (handler *AdminHandler) DeleteUser(writer http.ResponseWriter, request *http.Request) {
	targetID, parseError := uuid.Parse(chi.URLParam(request, "id"))
	if parseError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid user id")
		return
	}

	callerID := middleware.UserIDFromContext(request.Context())
	if err := handler.svc.DeleteUser(request.Context(), callerID, targetID); err != nil {
		if errors.Is(err, service.ErrSelfDeletion) {
			httputil.WriteError(writer, http.StatusBadRequest, "cannot delete your own account")
			return
		}
		httputil.WriteInternalError(writer, "internal error", err)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "user deleted"})
}

func (handler *AdminHandler) GetStats(writer http.ResponseWriter, request *http.Request) {
	stats, fetchError := handler.svc.GetStats(request.Context())
	if fetchError != nil {
		httputil.WriteInternalError(writer, "internal error", fetchError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, stats)
}

func (handler *AdminHandler) CleanupMessages(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Count int32 `json:"count"`
	}
	if err := httputil.DecodeJSON(request, &body); err != nil || body.Count <= 0 {
		httputil.WriteError(writer, http.StatusBadRequest, "count must be a positive integer")
		return
	}

	if err := handler.svc.DeleteOldestMessages(request.Context(), body.Count); err != nil {
		httputil.WriteInternalError(writer, "internal error", err)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "messages deleted"})
}

func (handler *AdminHandler) GetSettings(writer http.ResponseWriter, request *http.Request) {
	httputil.WriteJSON(writer, http.StatusOK, handler.svc.GetSettings())
}

func parsePagination(request *http.Request) (page, pageSize int) {
	page = 1
	pageSize = 50
	if value := request.URL.Query().Get("page"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if value := request.URL.Query().Get("page_size"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}
	return
}

func (handler *AdminHandler) ListMedia(writer http.ResponseWriter, request *http.Request) {
	page, pageSize := parsePagination(request)
	media, fetchError := handler.svc.ListMedia(request.Context(), page, pageSize)
	if fetchError != nil {
		httputil.WriteInternalError(writer, "internal error", fetchError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, media)
}

func (handler *AdminHandler) ListDeletedMedia(writer http.ResponseWriter, request *http.Request) {
	page, pageSize := parsePagination(request)
	media, fetchError := handler.svc.ListDeletedMedia(request.Context(), page, pageSize)
	if fetchError != nil {
		httputil.WriteInternalError(writer, "internal error", fetchError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, media)
}

func (handler *AdminHandler) GetMediaStats(writer http.ResponseWriter, request *http.Request) {
	stats, fetchError := handler.svc.GetMediaStats(request.Context())
	if fetchError != nil {
		httputil.WriteInternalError(writer, "internal error", fetchError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, stats)
}

func (handler *AdminHandler) DeleteMedia(writer http.ResponseWriter, request *http.Request) {
	id, parseError := uuid.Parse(chi.URLParam(request, "id"))
	if parseError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid media id")
		return
	}
	if err := handler.mediaSvc.DeleteMediaAdmin(request.Context(), id); err != nil {
		httputil.WriteInternalError(writer, "failed to delete media", err)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "media deleted"})
}

func (handler *AdminHandler) BulkDeleteMedia(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := httputil.DecodeJSON(request, &body); err != nil || len(body.IDs) == 0 {
		httputil.WriteError(writer, http.StatusBadRequest, "ids must be a non-empty array")
		return
	}
	deleted := 0
	for _, id := range body.IDs {
		if err := handler.mediaSvc.DeleteMediaAdmin(request.Context(), id); err == nil {
			deleted++
		}
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]int{"deleted": deleted})
}

func (handler *AdminHandler) BanUser(writer http.ResponseWriter, request *http.Request) {
	targetID, parseError := uuid.Parse(chi.URLParam(request, "id"))
	if parseError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid user id")
		return
	}

	var body struct {
		Banned bool `json:"banned"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(request.Context())
	if banError := handler.svc.BanUser(request.Context(), callerID, targetID, body.Banned); banError != nil {
		if errors.Is(banError, service.ErrSelfBan) {
			httputil.WriteError(writer, http.StatusBadRequest, "cannot ban yourself")
			return
		}
		httputil.WriteInternalError(writer, "internal error", banError)
		return
	}

	action := "banned"
	if !body.Banned {
		action = "unbanned"
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "user " + action})
}

func (handler *AdminHandler) DeleteUserMessages(writer http.ResponseWriter, request *http.Request) {
	targetID, parseError := uuid.Parse(chi.URLParam(request, "id"))
	if parseError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid user id")
		return
	}

	count, deleteError := handler.svc.DeleteUserMessages(request.Context(), targetID)
	if deleteError != nil {
		httputil.WriteInternalError(writer, "internal error", deleteError)
		return
	}

	httputil.WriteJSON(writer, http.StatusOK, map[string]int64{"deleted": count})
}

func (handler *AdminHandler) UpdateSettings(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		OpenRegistration *bool   `json:"open_registration"`
		InstanceName     *string `json:"instance_name"`
		MaxMessages      *int64  `json:"max_messages"`
		MaxMessageChars  *int    `json:"max_message_chars"`
	}
	if err := httputil.DecodeJSON(request, &body); err != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.MaxMessages != nil && *body.MaxMessages < 0 {
		httputil.WriteError(writer, http.StatusBadRequest, "max_messages must be >= 0")
		return
	}
	if body.MaxMessageChars != nil && (*body.MaxMessageChars < 1 || *body.MaxMessageChars > 10000) {
		httputil.WriteError(writer, http.StatusBadRequest, "max_message_chars must be between 1 and 10000")
		return
	}

	if err := handler.svc.UpdateSettings(request.Context(), body.OpenRegistration, body.InstanceName, body.MaxMessages, body.MaxMessageChars); err != nil {
		httputil.WriteInternalError(writer, "failed to update settings", err)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, handler.svc.GetSettings())
}

func (handler *AdminHandler) CreateInviteCode(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Code      string  `json:"code"`
		MaxUses   *int32  `json:"max_uses"`
		ExpiresAt *string `json:"expires_at"`
	}
	if decodeError := httputil.DecodeJSON(request, &body); decodeError != nil || body.Code == "" {
		httputil.WriteError(writer, http.StatusBadRequest, "code is required")
		return
	}

	callerID := middleware.UserIDFromContext(request.Context())

	var maxUses sql.NullInt32
	if body.MaxUses != nil {
		maxUses = sql.NullInt32{Int32: *body.MaxUses, Valid: true}
	}

	var expiresAt sql.NullTime
	if body.ExpiresAt != nil && *body.ExpiresAt != "" {
		parsed, parseError := time.Parse(time.RFC3339, *body.ExpiresAt)
		if parseError != nil {
			httputil.WriteError(writer, http.StatusBadRequest, "invalid expires_at format, use RFC3339")
			return
		}
		expiresAt = sql.NullTime{Time: parsed, Valid: true}
	}

	inviteCode, createError := handler.svc.CreateInviteCode(request.Context(), body.Code, maxUses, expiresAt, callerID)
	if createError != nil {
		httputil.WriteInternalError(writer, "failed to create invite code", createError)
		return
	}
	httputil.WriteJSON(writer, http.StatusCreated, inviteCode)
}

func (handler *AdminHandler) ListInviteCodes(writer http.ResponseWriter, request *http.Request) {
	codes, fetchError := handler.svc.ListInviteCodes(request.Context())
	if fetchError != nil {
		httputil.WriteInternalError(writer, "failed to list invite codes", fetchError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, codes)
}

func (handler *AdminHandler) DeleteInviteCode(writer http.ResponseWriter, request *http.Request) {
	codeID, parseError := uuid.Parse(chi.URLParam(request, "id"))
	if parseError != nil {
		httputil.WriteError(writer, http.StatusBadRequest, "invalid invite code id")
		return
	}
	if deleteError := handler.svc.DeleteInviteCode(request.Context(), codeID); deleteError != nil {
		httputil.WriteInternalError(writer, "failed to delete invite code", deleteError)
		return
	}
	httputil.WriteJSON(writer, http.StatusOK, map[string]string{"message": "invite code deleted"})
}
