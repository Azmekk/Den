package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/martinmckenna/den/internal/httputil"
	"github.com/martinmckenna/den/internal/service"
)

type ChannelHandler struct {
	svc *service.ChannelService
}

func NewChannelHandler(svc *service.ChannelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

type createChannelRequest struct {
	Name     string `json:"name"`
	Topic    string `json:"topic"`
	Position int32  `json:"position"`
}

type updateChannelRequest struct {
	Name     string `json:"name"`
	Topic    string `json:"topic"`
	Position int32  `json:"position"`
}

func (h *ChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.svc.List(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, channels)
}

func (h *ChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	ch, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "channel not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, ch)
}

func (h *ChannelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createChannelRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ch, err := h.svc.Create(r.Context(), req.Name, req.Topic, req.Position)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			httputil.WriteError(w, http.StatusBadRequest, "name must be 1-64 characters")
		case errors.Is(err, service.ErrChannelNameTaken):
			httputil.WriteError(w, http.StatusConflict, "channel name already taken")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, ch)
}

func (h *ChannelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	var req updateChannelRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ch, err := h.svc.Update(r.Context(), id, req.Name, req.Topic, req.Position)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrChannelNotFound):
			httputil.WriteError(w, http.StatusNotFound, "channel not found")
		case errors.Is(err, service.ErrInvalidInput):
			httputil.WriteError(w, http.StatusBadRequest, "name must be 1-64 characters")
		case errors.Is(err, service.ErrChannelNameTaken):
			httputil.WriteError(w, http.StatusConflict, "channel name already taken")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	httputil.WriteJSON(w, http.StatusOK, ch)
}

func (h *ChannelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "channel deleted"})
}
