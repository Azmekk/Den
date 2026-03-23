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
// The api parameter may be nil, in which case the default Pion API is used.
func NewParticipant(userID uuid.UUID, username string, room *Room, config webrtc.Configuration, api *webrtc.API, sendMessage func(data []byte)) (*Participant, error) {
	var peerConn *webrtc.PeerConnection
	var err error
	if api != nil {
		peerConn, err = api.NewPeerConnection(config)
	} else {
		peerConn, err = webrtc.NewPeerConnection(config)
	}
	if err != nil {
		log.Printf("voice: failed to create PeerConnection for %s (%s): %v", userID, username, err)
		return nil, err
	}

	log.Printf("voice: created PeerConnection for %s (%s) in room %s", userID, username, room.ChannelID)

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
			log.Printf("voice: [%s] ICE candidate gathering complete (nil sentinel)", userID)
			return
		}
		log.Printf("voice: [%s] server ICE candidate: type=%s addr=%s:%d protocol=%s",
			userID, candidate.Typ.String(), candidate.Address, candidate.Port, candidate.Protocol.String())
		candidateJSON := candidate.ToJSON()
		data, marshalErr := json.Marshal(map[string]any{
			"type":       "voice_ice_candidate",
			"channel_id": room.ChannelID.String(),
			"candidate":  candidateJSON,
		})
		if marshalErr != nil {
			log.Printf("voice: [%s] failed to marshal ICE candidate: %v", userID, marshalErr)
			return
		}
		sendMessage(data)
	})

	peerConn.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("voice: [%s] ICE connection state: %s", userID, state.String())
	})

	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("voice: [%s] peer connection state: %s", userID, state.String())
	})

	peerConn.OnICEGatheringStateChange(func(state webrtc.ICEGatheringState) {
		log.Printf("voice: [%s] ICE gathering state: %s", userID, state.String())
	})

	peerConn.OnSignalingStateChange(func(state webrtc.SignalingState) {
		log.Printf("voice: [%s] signaling state: %s", userID, state.String())
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

	log.Printf("voice: [%s] handling client offer (type=%s)", participant.UserID, offer.Type.String())

	if err := participant.peerConn.SetRemoteDescription(offer); err != nil {
		log.Printf("voice: [%s] failed to set remote description (offer): %v", participant.UserID, err)
		return nil, err
	}

	answer, err := participant.peerConn.CreateAnswer(nil)
	if err != nil {
		log.Printf("voice: [%s] failed to create answer: %v", participant.UserID, err)
		return nil, err
	}

	if err := participant.peerConn.SetLocalDescription(answer); err != nil {
		log.Printf("voice: [%s] failed to set local description (answer): %v", participant.UserID, err)
		return nil, err
	}

	log.Printf("voice: [%s] sending answer (type=%s)", participant.UserID, answer.Type.String())
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

	log.Printf("voice: [%s] handling client answer (type=%s)", participant.UserID, answer.Type.String())

	if err := participant.peerConn.SetRemoteDescription(answer); err != nil {
		log.Printf("voice: [%s] failed to set remote description (answer): %v", participant.UserID, err)
		return err
	}

	return nil
}

// AddICECandidate adds a trickle ICE candidate from the client.
func (participant *Participant) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	participant.mu.Lock()
	defer participant.mu.Unlock()

	if participant.closed {
		return ErrParticipantClosed
	}

	candidateStr := ""
	if candidate.Candidate != "" {
		candidateStr = candidate.Candidate
	}
	log.Printf("voice: [%s] adding client ICE candidate: %s", participant.UserID, candidateStr)

	if err := participant.peerConn.AddICECandidate(candidate); err != nil {
		log.Printf("voice: [%s] failed to add ICE candidate: %v", participant.UserID, err)
		return err
	}

	return nil
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

	log.Printf("voice: [%s] adding subscription track %s (stream=%s)", participant.UserID, localTrack.ID(), localTrack.StreamID())

	sender, err := participant.peerConn.AddTrack(localTrack)
	if err != nil {
		log.Printf("voice: [%s] failed to add track: %v", participant.UserID, err)
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

	log.Printf("voice: [%s] removing subscription track %s", participant.UserID, trackID)

	if err := participant.peerConn.RemoveTrack(sender); err != nil {
		log.Printf("voice: [%s] failed to remove track %s: %v", participant.UserID, trackID, err)
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
	log.Printf("voice: [%s] closing PeerConnection", participant.UserID)
	participant.peerConn.Close()
}

// negotiate creates a new SDP offer and sends it to the client for renegotiation.
// Must be called with participant.mu held.
func (participant *Participant) negotiate() error {
	log.Printf("voice: [%s] initiating server-side renegotiation", participant.UserID)

	offer, err := participant.peerConn.CreateOffer(nil)
	if err != nil {
		log.Printf("voice: [%s] failed to create renegotiation offer: %v", participant.UserID, err)
		return err
	}

	if err := participant.peerConn.SetLocalDescription(offer); err != nil {
		log.Printf("voice: [%s] failed to set local description (renegotiation offer): %v", participant.UserID, err)
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

	log.Printf("voice: [%s] sending renegotiation offer to client", participant.UserID)
	participant.sendMessage(data)
	return nil
}

// onTrack is called when the client publishes a new media track (audio or video).
func (participant *Participant) onTrack(remote *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	source := classifyTrack(remote)
	log.Printf("voice: [%s] published track %s (kind=%s, source=%s, streamID=%s, codec=%s)",
		participant.UserID, remote.ID(), remote.Kind().String(), source, remote.StreamID(), remote.Codec().MimeType)

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
