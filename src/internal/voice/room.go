package voice

import (
	"errors"
	"io"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

var ErrParticipantClosed = errors.New("participant is closed")

// Room represents a voice channel with active participants. It manages
// PeerConnections and forwards RTP packets between participants (SFU pattern).
type Room struct {
	ChannelID    uuid.UUID
	participants map[uuid.UUID]*Participant
	mu           sync.RWMutex
	webrtcConfig webrtc.Configuration

	// forwarders tracks active RTP forwarding goroutines.
	// Key: "{publisherID}:{remoteTrackID}". Closing the channel stops the forwarder.
	forwarders map[string]chan struct{}
}

// NewRoom creates an empty voice room for the given channel.
func NewRoom(channelID uuid.UUID, config webrtc.Configuration) *Room {
	return &Room{
		ChannelID:    channelID,
		participants: make(map[uuid.UUID]*Participant),
		webrtcConfig: config,
		forwarders:   make(map[string]chan struct{}),
	}
}

// AddParticipant creates a new Participant and subscribes them to all existing
// published tracks so they can immediately hear everyone already in the room.
func (room *Room) AddParticipant(userID uuid.UUID, username string, sendMessage func(data []byte)) (*Participant, error) {
	room.mu.Lock()
	defer room.mu.Unlock()

	// Close existing participant if rejoining
	if existing, ok := room.participants[userID]; ok {
		existing.Close()
		delete(room.participants, userID)
	}

	participant, err := NewParticipant(userID, username, room, room.webrtcConfig, sendMessage)
	if err != nil {
		return nil, err
	}

	room.participants[userID] = participant
	return participant, nil
}

// RemoveParticipant closes the participant's PeerConnection and removes their
// published tracks from all other participants.
func (room *Room) RemoveParticipant(userID uuid.UUID) {
	room.mu.Lock()
	defer room.mu.Unlock()

	participant, ok := room.participants[userID]
	if !ok {
		return
	}

	// Stop all forwarders for this publisher's tracks
	for forwarderKey, stopChan := range room.forwarders {
		if isForwarderForPublisher(forwarderKey, userID) {
			close(stopChan)
			delete(room.forwarders, forwarderKey)
		}
	}

	// Remove subscriptions from other participants
	participant.mu.Lock()
	publishedTrackIDs := make([]string, 0, len(participant.published))
	for trackID := range participant.published {
		publishedTrackIDs = append(publishedTrackIDs, trackID)
	}
	participant.mu.Unlock()

	for _, trackID := range publishedTrackIDs {
		for otherID, other := range room.participants {
			if otherID == userID {
				continue
			}
			if err := other.RemoveSubscription(trackID); err != nil {
				log.Printf("voice: failed to remove subscription from %s: %v", otherID, err)
			}
		}
	}

	participant.Close()
	delete(room.participants, userID)
}

// OnTrackPublished is called when a participant publishes a new media track.
// It creates a corresponding local track for each other participant, adds it
// as a subscription, and starts an RTP forwarding goroutine.
func (room *Room) OnTrackPublished(publisherID uuid.UUID, forwarded *ForwardedTrack) {
	room.mu.RLock()
	defer room.mu.RUnlock()

	// Collect subscribers and their local tracks
	var localTracks []*webrtc.TrackLocalStaticRTP

	for subscriberID, subscriber := range room.participants {
		if subscriberID == publisherID {
			continue
		}

		localTrack, err := newLocalTrackFromRemote(forwarded.Remote, forwarded.UserID, forwarded.Source)
		if err != nil {
			log.Printf("voice: failed to create local track for subscriber %s: %v", subscriberID, err)
			continue
		}

		if err := subscriber.AddSubscription(localTrack); err != nil {
			log.Printf("voice: failed to add subscription to %s: %v", subscriberID, err)
			continue
		}

		localTracks = append(localTracks, localTrack)
	}

	if len(localTracks) == 0 {
		// Still need to consume the track to avoid blocking
		go func() {
			buf := make([]byte, 1500)
			for {
				if _, _, err := forwarded.Remote.Read(buf); err != nil {
					return
				}
			}
		}()
		return
	}

	// Start forwarding RTP from the remote track to all local tracks
	forwarderKey := forwarderKeyForTrack(publisherID, forwarded.Remote.ID())
	stopChan := make(chan struct{})
	room.forwarders[forwarderKey] = stopChan

	go forwardRTP(forwarded.Remote, localTracks, stopChan)
}

// IsEmpty returns true if there are no participants in the room.
func (room *Room) IsEmpty() bool {
	room.mu.RLock()
	defer room.mu.RUnlock()
	return len(room.participants) == 0
}

// GetParticipant returns the participant with the given user ID, or nil.
func (room *Room) GetParticipant(userID uuid.UUID) *Participant {
	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.participants[userID]
}

// Close tears down all participants and stops all forwarders.
func (room *Room) Close() {
	room.mu.Lock()
	defer room.mu.Unlock()

	for _, stopChan := range room.forwarders {
		close(stopChan)
	}
	room.forwarders = make(map[string]chan struct{})

	for _, participant := range room.participants {
		participant.Close()
	}
	room.participants = make(map[uuid.UUID]*Participant)
}

// forwardRTP reads RTP packets from the remote track and writes them to all
// local tracks until the remote track ends or the stop channel is closed.
func forwardRTP(remote *webrtc.TrackRemote, localTracks []*webrtc.TrackLocalStaticRTP, stop chan struct{}) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-stop:
			return
		default:
		}

		readCount, _, readErr := remote.Read(buf)
		if readErr != nil {
			if readErr == io.EOF {
				return
			}
			return
		}

		for _, localTrack := range localTracks {
			if _, writeErr := localTrack.Write(buf[:readCount]); writeErr != nil && writeErr != io.ErrClosedPipe {
				// Track was removed, ignore
			}
		}
	}
}

func forwarderKeyForTrack(publisherID uuid.UUID, trackID string) string {
	return publisherID.String() + ":" + trackID
}

func isForwarderForPublisher(key string, publisherID uuid.UUID) bool {
	prefix := publisherID.String() + ":"
	return len(key) > len(prefix) && key[:len(prefix)] == prefix
}
