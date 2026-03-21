package voice

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

// ForwardedTrack represents a track published by one participant that gets
// forwarded to other participants. The Local track is what gets added to
// subscriber PeerConnections; the Remote track is the source of RTP packets.
type ForwardedTrack struct {
	UserID uuid.UUID
	Source string // "microphone", "screen", "screen_audio"
	Remote *webrtc.TrackRemote
}

// streamID builds the stream identifier that browsers read from
// RTCTrackEvent.streams[0].id to identify who a remote track belongs to.
func streamID(userID uuid.UUID, source string) string {
	return fmt.Sprintf("%s:%s", userID.String(), source)
}

// newLocalTrackFromRemote creates a TrackLocalStaticRTP that matches the codec
// of the given remote track. The StreamID encodes the publisher's identity so
// the browser can attribute the track to the correct user.
func newLocalTrackFromRemote(remote *webrtc.TrackRemote, userID uuid.UUID, source string) (*webrtc.TrackLocalStaticRTP, error) {
	return webrtc.NewTrackLocalStaticRTP(
		remote.Codec().RTPCodecCapability,
		remote.ID(),
		streamID(userID, source),
	)
}
