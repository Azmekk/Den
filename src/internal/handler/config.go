package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/voice"
)

type ConfigHandler struct {
	uploadsEnabled      bool
	voiceEnabled        bool
	voiceManager        *voice.Manager
	getMaxChars         func() int
	getOpenRegistration func() bool
	supabaseURL         string
	supabaseAnonKey     string
}

func NewConfigHandler(uploadsEnabled, voiceEnabled bool, voiceManager *voice.Manager, getMaxChars func() int, getOpenRegistration func() bool, supabaseURL, supabaseAnonKey string) *ConfigHandler {
	return &ConfigHandler{
		uploadsEnabled:      uploadsEnabled,
		voiceEnabled:        voiceEnabled,
		voiceManager:        voiceManager,
		getMaxChars:         getMaxChars,
		getOpenRegistration: getOpenRegistration,
		supabaseURL:         supabaseURL,
		supabaseAnonKey:     supabaseAnonKey,
	}
}

func (handler *ConfigHandler) GetConfig(writer http.ResponseWriter, request *http.Request) {
	config := map[string]any{
		"uploads_enabled":   handler.uploadsEnabled,
		"voice_enabled":     handler.voiceEnabled,
		"max_message_chars": handler.getMaxChars(),
		"open_registration": handler.getOpenRegistration(),
		"supabase_url":      handler.supabaseURL,
		"supabase_anon_key": handler.supabaseAnonKey,
	}

	if handler.voiceManager != nil {
		var iceServers []map[string]any
		for _, server := range handler.voiceManager.ICEServers {
			entry := map[string]any{"urls": server.URLs}
			if server.Username != "" {
				entry["username"] = server.Username
				entry["credential"] = server.Credential
			}
			iceServers = append(iceServers, entry)
		}
		config["ice_servers"] = iceServers
	}

	httputil.WriteJSON(writer, http.StatusOK, config)
}
