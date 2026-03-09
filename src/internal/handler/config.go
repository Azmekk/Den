package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
)

type ConfigHandler struct {
	uploadsEnabled bool
}

func NewConfigHandler(uploadsEnabled bool) *ConfigHandler {
	return &ConfigHandler{uploadsEnabled: uploadsEnabled}
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]bool{
		"uploads_enabled": h.uploadsEnabled,
	})
}
