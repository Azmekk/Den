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
	ICEServers   []webrtc.ICEServer
}

// NewManager creates a new voice Manager with the given ICE server configuration.
func NewManager(iceServers []webrtc.ICEServer) *Manager {
	config := webrtc.Configuration{
		ICEServers: iceServers,
	}

	return &Manager{
		rooms:        make(map[uuid.UUID]*Room),
		webrtcConfig: config,
		ICEServers:   iceServers,
	}
}

// JoinRoom adds a participant to a voice room, creating the room if it doesn't
// exist. Returns the Participant for WebRTC signaling.
func (manager *Manager) JoinRoom(channelID, userID uuid.UUID, username string, sendMessage func(data []byte)) (*Participant, error) {
	manager.mu.Lock()

	room, ok := manager.rooms[channelID]
	if !ok {
		room = NewRoom(channelID, manager.webrtcConfig)
		manager.rooms[channelID] = room
		log.Printf("voice: created room for channel %s", channelID)
	}

	manager.mu.Unlock()

	participant, err := room.AddParticipant(userID, username, sendMessage)
	if err != nil {
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
		return nil, err
	}

	participant := manager.getParticipant(channelID, userID)
	if participant == nil {
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
		return err
	}

	participant := manager.getParticipant(channelID, userID)
	if participant == nil {
		return ErrParticipantClosed
	}

	return participant.HandleAnswer(sdp)
}

// HandleICECandidate routes an ICE candidate from a client to the correct participant.
func (manager *Manager) HandleICECandidate(channelID, userID uuid.UUID, candidateJSON json.RawMessage) error {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateJSON, &candidate); err != nil {
		return err
	}

	participant := manager.getParticipant(channelID, userID)
	if participant == nil {
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
