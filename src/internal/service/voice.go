package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
)

type VoiceService struct {
	apiKey      string
	apiSecret   string
	publicURL   string
	internalURL string
}

// NewVoiceService creates a voice service. internalURL is the HTTP URL used for
// server-to-server LiveKit API calls (e.g. "http://livekit:7880" inside Docker).
// If internalURL is empty, it is derived from publicURL by converting ws:// to http://.
func NewVoiceService(apiKey, apiSecret, publicURL, internalURL string) *VoiceService {
	if apiKey == "" || apiSecret == "" || publicURL == "" {
		return nil
	}

	if internalURL == "" {
		// Fallback: derive from public URL (local dev without Docker)
		internalURL = publicURL
	}

	// Ensure the internal URL uses HTTP(S) for REST API calls
	internalURL = strings.Replace(internalURL, "wss://", "https://", 1)
	internalURL = strings.Replace(internalURL, "ws://", "http://", 1)

	return &VoiceService{
		apiKey:      apiKey,
		apiSecret:   apiSecret,
		publicURL:   publicURL,
		internalURL: internalURL,
	}
}

// EnsureRoom pre-creates the room on the LiveKit server so it's ready when
// the client's WebSocket connection arrives. LiveKit's CreateRoom is idempotent;
// calling it for an existing room is a no-op. This eliminates the race condition
// where a client connects to a stale/non-existent room and LiveKit hasn't
// finished initializing it yet.
func (s *VoiceService) EnsureRoom(ctx context.Context, roomName string) error {
	adminToken, err := s.generateAdminToken()
	if err != nil {
		return fmt.Errorf("generate admin token: %w", err)
	}

	body, err := json.Marshal(map[string]any{
		"name":             roomName,
		"empty_timeout":    60,
		"max_participants": 50,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := s.internalURL + "/twirp/livekit.RoomService/CreateRoom"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("call livekit CreateRoom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("livekit CreateRoom returned %d", resp.StatusCode)
	}

	return nil
}

func (s *VoiceService) generateAdminToken() (string, error) {
	at := auth.NewAccessToken(s.apiKey, s.apiSecret)
	grant := &auth.VideoGrant{RoomCreate: true}
	at.SetVideoGrant(grant).SetValidFor(30 * time.Second)
	return at.ToJWT()
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
