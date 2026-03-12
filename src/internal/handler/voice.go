package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type VoiceHandler struct {
	voiceSvc   *service.VoiceService
	channelSvc *service.ChannelService
}

func NewVoiceHandler(voiceSvc *service.VoiceService, channelSvc *service.ChannelService) *VoiceHandler {
	return &VoiceHandler{voiceSvc: voiceSvc, channelSvc: channelSvc}
}

func (h *VoiceHandler) Join(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "channelId"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	ch, err := h.channelSvc.Get(r.Context(), channelID)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "channel not found")
			return
		}
		httputil.WriteInternalError(w, "internal error", err)
		return
	}

	if !ch.IsVoice {
		httputil.WriteError(w, http.StatusBadRequest, "channel is not a voice channel")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	username := middleware.UsernameFromContext(r.Context())

	token, err := h.voiceSvc.GenerateToken(userID, username, channelID.String())
	if err != nil {
		httputil.WriteInternalError(w, "failed to generate token", err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"url":   h.voiceSvc.GetURL(),
	})
}
