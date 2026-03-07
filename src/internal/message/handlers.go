package message

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/martinmckenna/den/src/internal/httputil"
)

func (s *Service) GetHistoryHandler(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(r.PathValue("id"))
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

	messages, hasMore, err := s.GetHistory(r.Context(), channelID, beforeTime, beforeID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"has_more": hasMore,
	})
}
