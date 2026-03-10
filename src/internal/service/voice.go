package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
)

type VoiceService struct {
	apiKey    string
	apiSecret string
	url       string
}

func NewVoiceService(apiKey, apiSecret, url string) *VoiceService {
	if apiKey == "" || apiSecret == "" || url == "" {
		return nil
	}
	return &VoiceService{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		url:       url,
	}
}

func (s *VoiceService) GenerateToken(userID uuid.UUID, username, roomName string) (string, error) {
	at := auth.NewAccessToken(s.apiKey, s.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.SetVideoGrant(grant).
		SetIdentity(userID.String()).
		SetName(username).
		SetValidFor(time.Hour)

	return at.ToJWT()
}

func (s *VoiceService) GetURL() string {
	return s.url
}
