package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type DMHandler struct {
	svc *service.DMService
}

func NewDMHandler(svc *service.DMService) *DMHandler {
	return &DMHandler{svc: svc}
}

func (h *DMHandler) CreateOrGet(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var body struct {
		UserID string `json:"user_id"`
	}
	if err := httputil.DecodeJSON(r, &body); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}

	otherUserID, err := uuid.Parse(body.UserID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	pair, err := h.svc.CreateOrGetDMPair(r.Context(), userID, otherUserID)
	if err != nil {
		if err == service.ErrCannotDMSelf {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, pair)
}

func (h *DMHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	pairs, err := h.svc.ListConversations(r.Context(), userID)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, pairs)
}

func (h *DMHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	dmPairID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid dm pair id")
		return
	}

	// Validate user is in the DM pair
	if _, err := h.svc.ValidateUserInPair(r.Context(), dmPairID, userID); err != nil {
		if err == service.ErrNotInDMPair || err == service.ErrDMPairNotFound {
			httputil.WriteError(w, http.StatusForbidden, "access denied")
			return
		}
		httputil.WriteInternalError(w, "internal error", err)
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

	messages, hasMore, err := h.svc.GetDMHistory(r.Context(), dmPairID, beforeTime, beforeID)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"has_more": hasMore,
	})
}

func (h *DMHandler) GetPins(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	dmPairID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid dm pair id")
		return
	}

	if _, err := h.svc.ValidateUserInPair(r.Context(), dmPairID, userID); err != nil {
		if err == service.ErrNotInDMPair || err == service.ErrDMPairNotFound {
			httputil.WriteError(w, http.StatusForbidden, "access denied")
			return
		}
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	messages, err := h.svc.GetPinnedDMMessages(r.Context(), dmPairID)
	if err != nil {
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, messages)
}
