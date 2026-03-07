package channel

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/martinmckenna/den/src/internal/httputil"
)

type createRequest struct {
	Name     string `json:"name"`
	Topic    string `json:"topic"`
	Position int32  `json:"position"`
}

type updateRequest struct {
	Name     string `json:"name"`
	Topic    string `json:"topic"`
	Position int32  `json:"position"`
}

func (s *Service) ListHandler(w http.ResponseWriter, r *http.Request) {
	channels, err := s.List(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, channels)
}

func (s *Service) GetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	ch, err := s.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "channel not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, ch)
}

func (s *Service) CreateHandler(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ch, err := s.Create(r.Context(), req.Name, req.Topic, req.Position)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			httputil.WriteError(w, http.StatusBadRequest, "name must be 1-64 characters")
		case errors.Is(err, ErrNameTaken):
			httputil.WriteError(w, http.StatusConflict, "channel name already taken")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, ch)
}

func (s *Service) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	var req updateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ch, err := s.Update(r.Context(), id, req.Name, req.Topic, req.Position)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			httputil.WriteError(w, http.StatusNotFound, "channel not found")
		case errors.Is(err, ErrInvalidInput):
			httputil.WriteError(w, http.StatusBadRequest, "name must be 1-64 characters")
		case errors.Is(err, ErrNameTaken):
			httputil.WriteError(w, http.StatusConflict, "channel name already taken")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	httputil.WriteJSON(w, http.StatusOK, ch)
}

func (s *Service) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	if err := s.Delete(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "channel deleted"})
}
