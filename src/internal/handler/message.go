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
		httputil.WriteInternalError(w, "internal error", err)
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
		httputil.WriteInternalError(w, "internal error", err)
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
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	h.hub.BroadcastGlobal(data)
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MessageHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	channelStr := r.URL.Query().Get("channel")
	authorStr := r.URL.Query().Get("author")
	afterStr := r.URL.Query().Get("after")
	beforeStr := r.URL.Query().Get("before")

	if q == "" && channelStr == "" && authorStr == "" && afterStr == "" && beforeStr == "" {
		httputil.WriteError(w, http.StatusBadRequest, "at least one search filter is required")
		return
	}

	var query *string
	if q != "" {
		query = &q
	}

	var channelID *uuid.UUID
	if channelStr != "" {
		id, err := uuid.Parse(channelStr)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
			return
		}
		channelID = &id
	}

	var authorID *uuid.UUID
	if authorStr != "" {
		id, err := uuid.Parse(authorStr)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid author id")
			return
		}
		authorID = &id
	}

	var afterTime *time.Time
	if afterStr != "" {
		t, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid after time")
			return
		}
		afterTime = &t
	}

	var beforeTime *time.Time
	if beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid before time")
			return
		}
		beforeTime = &t
	}

	results, err := h.svc.SearchMessages(r.Context(), query, channelID, authorID, afterTime, beforeTime)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"results": results,
	})
}

func (h *MessageHandler) GetMessagesAround(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	messageID, err := uuid.Parse(r.URL.Query().Get("message_id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid message_id")
		return
	}

	messages, hasMoreBefore, hasMoreAfter, err := h.svc.GetMessagesAround(r.Context(), channelID, messageID)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"messages":        messages,
		"has_more_before": hasMoreBefore,
		"has_more_after":  hasMoreAfter,
	})
}

func (h *MessageHandler) GetNewer(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	afterTime, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("after_time"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid after_time")
		return
	}

	afterID, err := uuid.Parse(r.URL.Query().Get("after_id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid after_id")
		return
	}

	messages, hasMore, err := h.svc.GetNewer(r.Context(), channelID, afterTime, afterID)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"has_more": hasMore,
	})
}

func (h *MessageHandler) GetPinnedMessages(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	messages, err := h.svc.GetPinnedMessages(r.Context(), channelID)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, messages)
}
