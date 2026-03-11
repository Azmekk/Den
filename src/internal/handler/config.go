package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
)

type ConfigHandler struct {
	uploadsEnabled      bool
	voiceEnabled        bool
	getMaxChars         func() int
	getOpenRegistration func() bool
}

func NewConfigHandler(uploadsEnabled, voiceEnabled bool, getMaxChars func() int, getOpenRegistration func() bool) *ConfigHandler {
	return &ConfigHandler{uploadsEnabled: uploadsEnabled, voiceEnabled: voiceEnabled, getMaxChars: getMaxChars, getOpenRegistration: getOpenRegistration}
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"uploads_enabled":   h.uploadsEnabled,
		"voice_enabled":     h.voiceEnabled,
		"max_message_chars": h.getMaxChars(),
		"open_registration": h.getOpenRegistration(),
	})
}
