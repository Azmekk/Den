package voice

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

// Manager manages voice rooms across all channels. It maps channel UUIDs to
// Room instances and handles routing of WebRTC signaling messages.
type Manager struct {
	rooms        map[uuid.UUID]*Room
	mu           sync.RWMutex
	webrtcConfig webrtc.Configuration
	webrtcAPI    *webrtc.API
	ICEServers   []webrtc.ICEServer
}

// NewManager creates a new voice Manager with the given ICE server configuration.
// The api parameter may be nil, in which case the default Pion API is used.
func NewManager(iceServers []webrtc.ICEServer, api *webrtc.API) *Manager {
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	return &Manager{
		rooms:        make(map[uuid.UUID]*Room),
		webrtcConfig: config,
		webrtcAPI:    api,
		ICEServers:   iceServers,
	}
}

// JoinRoom adds a participant to a voice room, creating the room if it doesn't
// exist. Returns the Participant for WebRTC signaling.
func (manager *Manager) JoinRoom(channelID, userID uuid.UUID, username string, sendMessage func(data []byte)) (*Participant, error) {
	manager.mu.Lock()

	room, ok := manager.rooms[channelID]
	if !ok {
		room = NewRoom(channelID, manager.webrtcConfig, manager.webrtcAPI)
		manager.rooms[channelID] = room
		log.Printf("voice: created room for channel %s", channelID)
	}

	manager.mu.Unlock()

	participant, err := room.AddParticipant(userID, username, sendMessage)
	if err != nil {
		log.Printf("voice: failed to add participant %s (%s) to room %s: %v", userID, username, channelID, err)
		return nil, err
	}

	log.Printf("voice: user %s (%s) joined room %s", userID, username, channelID)
	return participant, nil
}

// LeaveRoom removes a participant from a voice room and cleans up the room
// if it becomes empty.
func (manager *Manager) LeaveRoom(channelID, userID uuid.UUID) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	room, ok := manager.rooms[channelID]
	if !ok {
		return
	}

	room.RemoveParticipant(userID)
	log.Printf("voice: user %s left room %s", userID, channelID)

	if room.IsEmpty() {
		room.Close()
		delete(manager.rooms, channelID)
		log.Printf("voice: room %s closed (empty)", channelID)
	}
}

// LeaveAllRooms removes a user from any room they're in. Called when the
// user's last WebSocket connection disconnects.
func (manager *Manager) LeaveAllRooms(userID uuid.UUID) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	for channelID, room := range manager.rooms {
		if room.GetParticipant(userID) != nil {
			room.RemoveParticipant(userID)
			log.Printf("voice: user %s removed from room %s (disconnect)", userID, channelID)

			if room.IsEmpty() {
				room.Close()
				delete(manager.rooms, channelID)
				log.Printf("voice: room %s closed (empty)", channelID)
			}
		}
	}
}

// HandleOffer routes an SDP offer from a client to the correct participant
// and returns the SDP answer.
func (manager *Manager) HandleOffer(channelID, userID uuid.UUID, sdpJSON json.RawMessage) ([]byte, error) {
	var sdp webrtc.SessionDescription
	if err := json.Unmarshal(sdpJSON, &sdp); err != nil {
		log.Printf("voice: failed to unmarshal offer SDP from %s: %v", userID, err)
		return nil, err
	}

	log.Printf("voice: routing offer from %s in channel %s", userID, channelID)

	participant := manager.getParticipant(channelID, userID)
	if participant == nil {
		log.Printf("voice: no participant found for %s in channel %s (offer dropped)", userID, channelID)
		return nil, ErrParticipantClosed
	}

	answer, err := participant.HandleOffer(sdp)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(map[string]any{
		"type":       "voice_answer",
		"channel_id": channelID.String(),
		"sdp": map[string]string{
			"type": answer.Type.String(),
			"sdp":  answer.SDP,
		},
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}

// HandleAnswer routes an SDP answer from a client to the correct participant.
func (manager *Manager) HandleAnswer(channelID, userID uuid.UUID, sdpJSON json.RawMessage) error {
	var sdp webrtc.SessionDescription
	if err := json.Unmarshal(sdpJSON, &sdp); err != nil {
		log.Printf("voice: failed to unmarshal answer SDP from %s: %v", userID, err)
		return err
	}

	log.Printf("voice: routing answer from %s in channel %s", userID, channelID)

	participant := manager.getParticipant(channelID, userID)
	if participant == nil {
		log.Printf("voice: no participant found for %s in channel %s (answer dropped)", userID, channelID)
		return ErrParticipantClosed
	}

	return participant.HandleAnswer(sdp)
}

// HandleICECandidate routes an ICE candidate from a client to the correct participant.
func (manager *Manager) HandleICECandidate(channelID, userID uuid.UUID, candidateJSON json.RawMessage) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateJSON, &candidate); err != nil {
		log.Printf("voice: failed to unmarshal ICE candidate from %s: %v", userID, err)
		return err
	}

	participant := manager.getParticipant(channelID, userID)
	if participant == nil {
		log.Printf("voice: no participant found for %s in channel %s (ICE candidate dropped)", userID, channelID)
		return ErrParticipantClosed
	}

	return participant.AddICECandidate(candidate)
}

func (manager *Manager) getParticipant(channelID, userID uuid.UUID) *Participant {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	room, ok := manager.rooms[channelID]
	if !ok {
		return nil
	}

	return room.GetParticipant(userID)
}
