package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/ws"
)

type EmoteHandler struct {
	svc *service.EmoteService
	hub *ws.Hub
}

func NewEmoteHandler(svc *service.EmoteService, hub *ws.Hub) *EmoteHandler {
	return &EmoteHandler{svc: svc, hub: hub}
}

func (h *EmoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.svc.IsConfigured() {
		httputil.WriteError(w, http.StatusNotImplemented, "uploads not configured")
		return
	}

	if err := r.ParseMultipartForm(512 * 1024); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	name := r.FormValue("name")
	file, _, err := r.FormFile("image")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "missing image file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	uploaderID := middleware.UserIDFromContext(r.Context())
	emote, err := h.svc.Create(r.Context(), name, uploaderID, data, "")
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmoteNameInvalid):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrEmoteNameTaken):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrEmoteTooLarge):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrEmoteBadFormat):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrEmoteDimensions):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	h.hub.BroadcastGlobal([]byte(`{"type":"emote_list_update"}`))
	httputil.WriteJSON(w, http.StatusCreated, emote)
}

func (h *EmoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.svc.IsConfigured() {
		httputil.WriteError(w, http.StatusNotImplemented, "uploads not configured")
		return
	}

	emoteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid emote id")
		return
	}

	if err := h.svc.Delete(r.Context(), emoteID); err != nil {
		if errors.Is(err, service.ErrEmoteNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "emote not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.hub.BroadcastGlobal([]byte(`{"type":"emote_list_update"}`))
	w.WriteHeader(http.StatusNoContent)
}

func (h *EmoteHandler) List(w http.ResponseWriter, r *http.Request) {
	emotes, err := h.svc.List(r.Context())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, emotes)
}

func (h *EmoteHandler) ServeImage(w http.ResponseWriter, r *http.Request) {
	emoteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid emote id")
		return
	}

	url, err := h.svc.GetImageURL(r.Context(), emoteID)
	if err != nil {
		if errors.Is(err, service.ErrEmoteNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "emote not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.Redirect(w, r, url, http.StatusFound)
}
