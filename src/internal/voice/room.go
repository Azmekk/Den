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

// activeForwarder reads RTP from a single remote track and writes to a
// dynamically updatable set of local tracks. This allows new subscribers
// to be added after the forwarder is already running.
type activeForwarder struct {
	remote      *webrtc.TrackRemote
	localTracks []*webrtc.TrackLocalStaticRTP
	mu          sync.RWMutex
	stop        chan struct{}
}

func (forwarder *activeForwarder) addLocalTrack(track *webrtc.TrackLocalStaticRTP) {
	forwarder.mu.Lock()
	forwarder.localTracks = append(forwarder.localTracks, track)
	forwarder.mu.Unlock()
}

// Room represents a voice channel with active participants. It manages
// PeerConnections and forwards RTP packets between participants (SFU pattern).
type Room struct {
	ChannelID    uuid.UUID
	participants map[uuid.UUID]*Participant
	mu           sync.RWMutex
	webrtcConfig webrtc.Configuration
	webrtcAPI    *webrtc.API

	// forwarders tracks active RTP forwarding goroutines.
	// Key: "{publisherID}:{remoteTrackID}".
	forwarders map[string]*activeForwarder
}

// NewRoom creates an empty voice room for the given channel.
func NewRoom(channelID uuid.UUID, config webrtc.Configuration, api *webrtc.API) *Room {
	return &Room{
		ChannelID:    channelID,
		participants: make(map[uuid.UUID]*Participant),
		webrtcConfig: config,
		webrtcAPI:    api,
		forwarders:   make(map[string]*activeForwarder),
	}
}

// AddParticipant creates a new Participant and subscribes them to all existing
// published tracks so they can immediately hear everyone already in the room.
func (room *Room) AddParticipant(userID uuid.UUID, username string, sendMessage func(data []byte)) (*Participant, error) {
	room.mu.Lock()
	defer room.mu.Unlock()

	// Close existing participant if rejoining
	if existing, ok := room.participants[userID]; ok {
		log.Printf("voice: [room %s] closing existing participant %s (rejoin)", room.ChannelID, userID)
		existing.Close()
		delete(room.participants, userID)
	}

	participant, err := NewParticipant(userID, username, room, room.webrtcConfig, room.webrtcAPI, sendMessage)
	if err != nil {
		return nil, err
	}

	room.participants[userID] = participant

	// Subscribe the new participant to all existing published tracks from other users
	for publisherID, publisher := range room.participants {
		if publisherID == userID {
			continue
		}

		publisher.mu.Lock()
		publishedTracks := make([]*ForwardedTrack, 0, len(publisher.published))
		for _, forwarded := range publisher.published {
			publishedTracks = append(publishedTracks, forwarded)
		}
		publisher.mu.Unlock()

		// Pre-add tracks without triggering renegotiation. The tracks will be
		// included in the server's answer to the client's first offer, avoiding
		// a signaling collision where the server sends a renegotiation offer
		// before the client has completed its initial SDP exchange.
		participant.mu.Lock()
		for _, forwarded := range publishedTracks {
			localTrack, trackErr := newLocalTrackFromRemote(forwarded.Remote, forwarded.UserID, forwarded.Source)
			if trackErr != nil {
				log.Printf("voice: [room %s] failed to create local track for late joiner %s: %v", room.ChannelID, userID, trackErr)
				continue
			}

			log.Printf("voice: [room %s] pre-adding track for late joiner %s from %s (track=%s stream=%s)",
				room.ChannelID, userID, publisherID, localTrack.ID(), localTrack.StreamID())

			if subErr := participant.addTrackWithoutNegotiation(localTrack); subErr != nil {
				log.Printf("voice: [room %s] failed to pre-add track for late joiner %s: %v", room.ChannelID, userID, subErr)
				continue
			}

			// Add the local track to the existing forwarder so it receives RTP
			forwarderKey := forwarderKeyForTrack(publisherID, forwarded.Remote.ID())
			if fwd, ok := room.forwarders[forwarderKey]; ok {
				fwd.addLocalTrack(localTrack)
				log.Printf("voice: [room %s] added late joiner %s to forwarder %s", room.ChannelID, userID, forwarderKey)
			}
		}
		participant.mu.Unlock()
	}

	log.Printf("voice: [room %s] participant count: %d", room.ChannelID, len(room.participants))
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
	for forwarderKey, fwd := range room.forwarders {
		if isForwarderForPublisher(forwarderKey, userID) {
			log.Printf("voice: [room %s] stopping forwarder %s", room.ChannelID, forwarderKey)
			close(fwd.stop)
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
			if removeErr := other.RemoveSubscription(trackID); removeErr != nil {
				log.Printf("voice: [room %s] failed to remove subscription from %s: %v", room.ChannelID, otherID, removeErr)
			}
		}
	}

	participant.Close()
	delete(room.participants, userID)
	log.Printf("voice: [room %s] participant count after remove: %d", room.ChannelID, len(room.participants))
}

// OnTrackPublished is called when a participant publishes a new media track.
// It creates a corresponding local track for each other participant, adds it
// as a subscription, and starts an RTP forwarding goroutine.
func (room *Room) OnTrackPublished(publisherID uuid.UUID, forwarded *ForwardedTrack) {
	room.mu.RLock()
	defer room.mu.RUnlock()

	log.Printf("voice: [room %s] track published by %s: kind=%s source=%s, distributing to %d other participant(s)",
		room.ChannelID, publisherID, forwarded.Remote.Kind().String(), forwarded.Source, len(room.participants)-1)

	// Create the forwarder immediately — even with 0 subscribers.
	// New subscribers can be added dynamically when they join later.
	forwarderKey := forwarderKeyForTrack(publisherID, forwarded.Remote.ID())
	fwd := &activeForwarder{
		remote: forwarded.Remote,
		stop:   make(chan struct{}),
	}

	// Create local tracks for all current subscribers
	for subscriberID, subscriber := range room.participants {
		if subscriberID == publisherID {
			continue
		}

		localTrack, err := newLocalTrackFromRemote(forwarded.Remote, forwarded.UserID, forwarded.Source)
		if err != nil {
			log.Printf("voice: [room %s] failed to create local track for subscriber %s: %v", room.ChannelID, subscriberID, err)
			continue
		}

		log.Printf("voice: [room %s] adding subscription: %s -> subscriber %s (track=%s stream=%s)",
			room.ChannelID, publisherID, subscriberID, localTrack.ID(), localTrack.StreamID())

		if err := subscriber.AddSubscription(localTrack); err != nil {
			log.Printf("voice: [room %s] failed to add subscription to %s: %v", room.ChannelID, subscriberID, err)
			continue
		}

		fwd.localTracks = append(fwd.localTracks, localTrack)
	}

	room.forwarders[forwarderKey] = fwd

	log.Printf("voice: [room %s] starting RTP forwarder %s -> %d subscriber(s)", room.ChannelID, forwarderKey, len(fwd.localTracks))
	go runForwarder(fwd, room.ChannelID, forwarderKey)
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

	log.Printf("voice: [room %s] closing room (%d forwarders, %d participants)", room.ChannelID, len(room.forwarders), len(room.participants))

	for _, fwd := range room.forwarders {
		close(fwd.stop)
	}
	room.forwarders = make(map[string]*activeForwarder)

	for _, participant := range room.participants {
		participant.Close()
	}
	room.participants = make(map[uuid.UUID]*Participant)
}

// runForwarder reads RTP packets from the remote track and writes them to all
// local tracks. The local tracks list is read under a RWMutex so new subscribers
// can be added dynamically while the forwarder is running.
func runForwarder(fwd *activeForwarder, channelID uuid.UUID, forwarderKey string) {
	buf := make([]byte, 1500)
	packetsForwarded := 0
	for {
		select {
		case <-fwd.stop:
			log.Printf("voice: [room %s] RTP forwarder %s stopped (signal), forwarded %d packets", channelID, forwarderKey, packetsForwarded)
			return
		default:
		}

		readCount, _, readErr := fwd.remote.Read(buf)
		if readErr != nil {
			if readErr == io.EOF {
				log.Printf("voice: [room %s] RTP forwarder %s ended (EOF), forwarded %d packets", channelID, forwarderKey, packetsForwarded)
			} else {
				log.Printf("voice: [room %s] RTP forwarder %s read error: %v (forwarded %d packets)", channelID, forwarderKey, readErr, packetsForwarded)
			}
			return
		}

		fwd.mu.RLock()
		for _, localTrack := range fwd.localTracks {
			if _, writeErr := localTrack.Write(buf[:readCount]); writeErr != nil && writeErr != io.ErrClosedPipe {
				log.Printf("voice: [room %s] RTP forwarder %s write error to track %s: %v", channelID, forwarderKey, localTrack.ID(), writeErr)
			}
		}
		fwd.mu.RUnlock()
		packetsForwarded++
	}
}

func forwarderKeyForTrack(publisherID uuid.UUID, trackID string) string {
	return publisherID.String() + ":" + trackID
}

func isForwarderForPublisher(key string, publisherID uuid.UUID) bool {
	prefix := publisherID.String() + ":"
	return len(key) > len(prefix) && key[:len(prefix)] == prefix
}
