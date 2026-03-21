package voice

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

// Participant represents a single user's PeerConnection in a voice Room.
type Participant struct {
	UserID      uuid.UUID
	Username    string
	Room        *Room
	peerConn    *webrtc.PeerConnection
	sendMessage func(data []byte)

	// published tracks keyed by remote track ID
	published map[string]*ForwardedTrack

	// senders we added for other participants' tracks, keyed by local track ID
	senders map[string]*webrtc.RTPSender

	mu     sync.Mutex
	closed bool
}

// NewParticipant creates a PeerConnection for the given user and wires up
// the OnTrack, OnICECandidate, and OnICEConnectionStateChange callbacks.
func NewParticipant(userID uuid.UUID, username string, room *Room, config webrtc.Configuration, sendMessage func(data []byte)) (*Participant, error) {
	peerConn, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	participant := &Participant{
		UserID:      userID,
		Username:    username,
		Room:        room,
		peerConn:    peerConn,
		sendMessage: sendMessage,
		published:   make(map[string]*ForwardedTrack),
		senders:     make(map[string]*webrtc.RTPSender),
	}

	peerConn.OnTrack(participant.onTrack)

	peerConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		candidateJSON := candidate.ToJSON()
		data, marshalErr := json.Marshal(map[string]any{
			"type":       "voice_ice_candidate",
			"channel_id": room.ChannelID.String(),
			"candidate":  candidateJSON,
		})
		if marshalErr != nil {
			return
		}
		sendMessage(data)
	})

	peerConn.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("voice: participant %s ICE state: %s", userID, state.String())
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateDisconnected {
			// Client will detect this and reconnect
		}
	})

	return participant, nil
}

// HandleOffer sets the remote SDP offer from the client and returns the
// server's SDP answer.
func (participant *Participant) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return nil, ErrParticipantClosed
	}

	if err := participant.peerConn.SetRemoteDescription(offer); err != nil {
		return nil, err
	}

	answer, err := participant.peerConn.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	if err := participant.peerConn.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	return &answer, nil
}

// HandleAnswer sets the remote SDP answer from the client (in response to
// a server-initiated renegotiation offer).
func (participant *Participant) HandleAnswer(answer webrtc.SessionDescription) error {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return ErrParticipantClosed
	}

	return participant.peerConn.SetRemoteDescription(answer)
}

// AddICECandidate adds a trickle ICE candidate from the client.
func (participant *Participant) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return ErrParticipantClosed
	}

	return participant.peerConn.AddICECandidate(candidate)
}

// AddSubscription adds a remote participant's track to this participant's
// PeerConnection so they can hear/see the remote participant. It then triggers
// a renegotiation so the client learns about the new track.
func (participant *Participant) AddSubscription(localTrack *webrtc.TrackLocalStaticRTP) error {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return ErrParticipantClosed
	}

	sender, err := participant.peerConn.AddTrack(localTrack)
	if err != nil {
		return err
	}

	participant.senders[localTrack.ID()] = sender

	// Consume RTCP from the sender (required by Pion to avoid blocking)
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, readErr := sender.Read(rtcpBuf); readErr != nil {
				return
			}
		}
	}()

	return participant.negotiate()
}

// RemoveSubscription removes a previously added subscription track.
func (participant *Participant) RemoveSubscription(trackID string) error {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return ErrParticipantClosed
	}

	sender, ok := participant.senders[trackID]
	if !ok {
		return nil
	}

	if err := participant.peerConn.RemoveTrack(sender); err != nil {
		return err
	}
	delete(participant.senders, trackID)

	return participant.negotiate()
}

// Close tears down the PeerConnection and marks the participant as closed.
func (participant *Participant) Close() {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return
	}
	participant.closed = true
	participant.peerConn.Close()
}

// negotiate creates a new SDP offer and sends it to the client for renegotiation.
// Must be called with participant.mu held.
func (participant *Participant) negotiate() error {
	offer, err := participant.peerConn.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err := participant.peerConn.SetLocalDescription(offer); err != nil {
		return err
	}

	data, err := json.Marshal(map[string]any{
		"type":       "voice_offer",
		"channel_id": participant.Room.ChannelID.String(),
		"sdp": map[string]string{
			"type": "offer",
			"sdp":  offer.SDP,
		},
	})
	if err != nil {
		return err
	}

	participant.sendMessage(data)
	return nil
}

// onTrack is called when the client publishes a new media track (audio or video).
func (participant *Participant) onTrack(remote *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	source := classifyTrack(remote)
	log.Printf("voice: participant %s published track %s (kind=%s, source=%s)",
		participant.UserID, remote.ID(), remote.Kind().String(), source)

	forwarded := &ForwardedTrack{
		UserID: participant.UserID,
		Source: source,
		Remote: remote,
	}

	participant.mu.Lock()
	participant.published[remote.ID()] = forwarded
	participant.mu.Unlock()

	// Send PLI requests periodically for video tracks to get keyframes
	if remote.Kind() == webrtc.RTPCodecTypeVideo {
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				writeErr := participant.peerConn.WriteRTCP([]rtcp.Packet{
					&rtcp.PictureLossIndication{MediaSSRC: uint32(remote.SSRC())},
				})
				if writeErr != nil {
					return
				}
			}
		}()
	}

	// Notify room — this will create local tracks for all other participants
	// and start forwarding RTP packets.
	participant.Room.OnTrackPublished(participant.UserID, forwarded)
}

// classifyTrack determines the source of a track based on its stream ID
// and codec type. The client sets stream IDs to help classify.
func classifyTrack(remote *webrtc.TrackRemote) string {
	streamID := remote.StreamID()

	// Check for screen share markers
	if remote.Kind() == webrtc.RTPCodecTypeVideo {
		return "screen"
	}

	// Audio tracks: if stream ID contains "screen", it's screen share audio
	if len(streamID) > 0 {
		for _, marker := range []string{"screen", "display"} {
			if containsIgnoreCase(streamID, marker) {
				return "screen_audio"
			}
		}
	}

	return "microphone"
}

func containsIgnoreCase(haystack, needle string) bool {
	for index := 0; index <= len(haystack)-len(needle); index++ {
		match := true
		for charIndex := 0; charIndex < len(needle); charIndex++ {
			haystackChar := haystack[index+charIndex]
			needleChar := needle[charIndex]
			if haystackChar != needleChar && haystackChar != needleChar^0x20 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

