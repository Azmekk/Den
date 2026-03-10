package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
)

type ConfigHandler struct {
	uploadsEnabled bool
	voiceEnabled   bool
}

func NewConfigHandler(uploadsEnabled, voiceEnabled bool) *ConfigHandler {
	return &ConfigHandler{uploadsEnabled: uploadsEnabled, voiceEnabled: voiceEnabled}
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"uploads_enabled": h.uploadsEnabled,
		"voice_enabled":   h.voiceEnabled,
	})
}
