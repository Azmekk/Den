package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/martinmckenna/den/internal/httputil"
	"github.com/martinmckenna/den/internal/service"
)

type MessageHandler struct {
	svc *service.MessageService
}

func NewMessageHandler(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
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
