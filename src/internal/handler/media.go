package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type MediaHandler struct {
	svc *service.MediaService
}

func NewMediaHandler(svc *service.MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

func (h *MediaHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if !h.svc.IsConfigured() {
		httputil.WriteError(w, http.StatusNotImplemented, "uploads not configured")
		return
	}

	if err := r.ParseMultipartForm(25 * 1024 * 1024); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 25*1024*1024+1))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	uploaderID := middleware.UserIDFromContext(r.Context())
	url, err := h.svc.UploadImage(r.Context(), uploaderID, data)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaTooLarge):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrMediaBadFormat):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteInternalError(w, "upload failed", err)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *MediaHandler) UploadVideo(w http.ResponseWriter, r *http.Request) {
	if !h.svc.IsConfigured() {
		httputil.WriteError(w, http.StatusNotImplemented, "uploads not configured")
		return
	}

	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 100*1024*1024+1))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
		return
	}

	uploaderID := middleware.UserIDFromContext(r.Context())
	url, err := h.svc.UploadVideo(r.Context(), uploaderID, data)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMediaTooLarge):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrMediaBadFormat):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteInternalError(w, "internal error", err)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
}
