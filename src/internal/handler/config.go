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
	supabaseURL         string
	supabaseAnonKey     string
}

func NewConfigHandler(uploadsEnabled, voiceEnabled bool, getMaxChars func() int, getOpenRegistration func() bool, supabaseURL, supabaseAnonKey string) *ConfigHandler {
	return &ConfigHandler{
		uploadsEnabled:      uploadsEnabled,
		voiceEnabled:        voiceEnabled,
		getMaxChars:         getMaxChars,
		getOpenRegistration: getOpenRegistration,
		supabaseURL:         supabaseURL,
		supabaseAnonKey:     supabaseAnonKey,
	}
}

func (handler *ConfigHandler) GetConfig(writer http.ResponseWriter, request *http.Request) {
	httputil.WriteJSON(writer, http.StatusOK, map[string]any{
		"uploads_enabled":   handler.uploadsEnabled,
		"voice_enabled":     handler.voiceEnabled,
		"max_message_chars": handler.getMaxChars(),
		"open_registration": handler.getOpenRegistration(),
		"supabase_url":      handler.supabaseURL,
		"supabase_anon_key": handler.supabaseAnonKey,
	})
}
