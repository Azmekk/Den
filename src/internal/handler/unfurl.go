package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/service"
)

type UnfurlHandler struct {
	svc *service.UnfurlService
}

func NewUnfurlHandler(svc *service.UnfurlService) *UnfurlHandler {
	return &UnfurlHandler{svc: svc}
}

func (h *UnfurlHandler) Unfurl(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		httputil.WriteError(w, http.StatusBadRequest, "url parameter required")
		return
	}

	result, err := h.svc.Unfurl(rawURL)
	if err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, "could not unfurl URL")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=600")
	httputil.WriteJSON(w, http.StatusOK, result)
}
