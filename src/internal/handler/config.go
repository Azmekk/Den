package handler

import (
	"net/http"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/voice"
)

type ConfigHandler struct {
	uploadsEnabled      bool
	bucketPublicURL     string
	voiceEnabled        bool
	voiceManager        *voice.Manager
	getMaxChars         func() int
	getOpenRegistration func() bool
	smtpEnabled         bool
}

func NewConfigHandler(uploadsEnabled, voiceEnabled bool, bucketPublicURL string, voiceManager *voice.Manager, getMaxChars func() int, getOpenRegistration func() bool, smtpEnabled bool) *ConfigHandler {
	return &ConfigHandler{
		uploadsEnabled:      uploadsEnabled,
		bucketPublicURL:     bucketPublicURL,
		voiceEnabled:        voiceEnabled,
		voiceManager:        voiceManager,
		getMaxChars:         getMaxChars,
		getOpenRegistration: getOpenRegistration,
		smtpEnabled:         smtpEnabled,
	}
}

func (handler *ConfigHandler) GetConfig(writer http.ResponseWriter, request *http.Request) {
	config := map[string]any{
		"uploads_enabled":   handler.uploadsEnabled,
		"voice_enabled":     handler.voiceEnabled,
		"max_message_chars": handler.getMaxChars(),
		"open_registration": handler.getOpenRegistration(),
		"smtp_enabled":      handler.smtpEnabled,
	}

	if handler.bucketPublicURL != "" {
		config["bucket_public_url"] = handler.bucketPublicURL
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
