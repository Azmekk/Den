package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
)

type ConfigHandler struct {
	uploadsEnabled bool
	voiceEnabled   bool
	getMaxChars    func() int
}

func NewConfigHandler(uploadsEnabled, voiceEnabled bool, getMaxChars func() int) *ConfigHandler {
	return &ConfigHandler{uploadsEnabled: uploadsEnabled, voiceEnabled: voiceEnabled, getMaxChars: getMaxChars}
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"uploads_enabled":  h.uploadsEnabled,
		"voice_enabled":    h.voiceEnabled,
		"max_message_chars": h.getMaxChars(),
	})
}
