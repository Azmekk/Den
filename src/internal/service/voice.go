package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
)

type VoiceService struct {
	apiKey    string
	apiSecret string
	publicURL string
}

func NewVoiceService(apiKey, apiSecret, publicURL string) *VoiceService {
	if apiKey == "" || apiSecret == "" || publicURL == "" {
		return nil
	}
	return &VoiceService{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		publicURL: publicURL,
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
	return s.publicURL
}
