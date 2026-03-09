package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/ws"
)

type MessageHandler struct {
	svc *service.MessageService
	hub *ws.Hub
}

func NewMessageHandler(svc *service.MessageService, hub *ws.Hub) *MessageHandler {
	return &MessageHandler{svc: svc, hub: hub}
}

func (h *MessageHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	var beforeTime *time.Time
	var beforeID *uuid.UUID

	if bt := r.URL.Query().Get("before_time"); bt != "" {
		t, err := time.Parse(time.RFC3339Nano, bt)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid before_time")
			return
		}
		beforeTime = &t
	}

	if bi := r.URL.Query().Get("before_id"); bi != "" {
		id, err := uuid.Parse(bi)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid before_id")
			return
		}
		beforeID = &id
	}

	if (beforeTime == nil) != (beforeID == nil) {
		httputil.WriteError(w, http.StatusBadRequest, "before_time and before_id must both be provided")
		return
	}

	messages, hasMore, err := h.svc.GetHistory(r.Context(), channelID, beforeTime, beforeID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"has_more": hasMore,
	})
}

func (h *MessageHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	isAdmin := middleware.IsAdminFromContext(r.Context())

	messageID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	data, err := h.svc.PinMessage(r.Context(), messageID, userID, isAdmin)
	if err != nil {
		if err == service.ErrMessageNotFound {
			httputil.WriteError(w, http.StatusNotFound, "message not found")
			return
		}
		if err == service.ErrForbidden {
			httputil.WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.hub.BroadcastGlobal(data)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MessageHandler) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	isAdmin := middleware.IsAdminFromContext(r.Context())

	messageID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	data, err := h.svc.UnpinMessage(r.Context(), messageID, userID, isAdmin)
	if err != nil {
		if err == service.ErrMessageNotFound {
			httputil.WriteError(w, http.StatusNotFound, "message not found")
			return
		}
		if err == service.ErrForbidden {
			httputil.WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.hub.BroadcastGlobal(data)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MessageHandler) GetPinnedMessages(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	messages, err := h.svc.GetPinnedMessages(r.Context(), channelID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, messages)
}
