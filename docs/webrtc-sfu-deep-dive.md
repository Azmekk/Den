# Building a WebRTC SFU from Scratch: A Deep Dive

This document explains every technology, protocol, and trick used to build a custom WebRTC Selective Forwarding Unit (SFU) for real-time voice and video, replacing an external LiveKit server with a Pion-based SFU embedded directly in a Go backend. The frontend uses native browser `RTCPeerConnection` APIs instead of a third-party SDK, with all signaling routed over an existing WebSocket connection.

---

## Table of Contents

1. [What is WebRTC?](#1-what-is-webrtc)
2. [SFU Architecture vs. Mesh vs. MCU](#2-sfu-architecture-vs-mesh-vs-mcu)
3. [The Signaling Layer: SDP and ICE](#3-the-signaling-layer-sdp-and-ice)
4. [Pion WebRTC: A Go-Native WebRTC Stack](#4-pion-webrtc-a-go-native-webrtc-stack)
5. [Server-Side Architecture](#5-server-side-architecture)
6. [RTP Forwarding: The Core of the SFU](#6-rtp-forwarding-the-core-of-the-sfu)
7. [Track Identity: The StreamID Trick](#7-track-identity-the-streamid-trick)
8. [Server-Initiated Renegotiation](#8-server-initiated-renegotiation)
9. [ICE: NAT Traversal with STUN and TURN](#9-ice-nat-traversal-with-stun-and-turn)
10. [The RTCP Drain Goroutine](#10-the-rtcp-drain-goroutine)
11. [PLI: Getting Keyframes for Video](#11-pli-getting-keyframes-for-video)
12. [Client-Side Audio Processing Pipeline](#12-client-side-audio-processing-pipeline)
13. [Mono-to-Stereo Upmixing for Echo Cancellation](#13-mono-to-stereo-upmixing-for-echo-cancellation)
14. [Signaling Over an Existing WebSocket](#14-signaling-over-an-existing-websocket)
15. [ICE Candidate Buffering (Trickle ICE)](#15-ice-candidate-buffering-trickle-ice)
16. [Mute and Speaking State Without a Media Server](#16-mute-and-speaking-state-without-a-media-server)
17. [Screen Sharing via addTrack](#17-screen-sharing-via-addtrack)
18. [Lifecycle and Cleanup](#18-lifecycle-and-cleanup)
19. [Technologies Used](#19-technologies-used)

---

## 1. What is WebRTC?

WebRTC (Web Real-Time Communication) is a set of browser APIs and protocols that enable peer-to-peer audio, video, and data transfer without plugins. Under the hood, it is a stack of protocols:

- **SRTP** (Secure Real-time Transport Protocol) carries encrypted media (audio/video packets)
- **RTCP** (RTP Control Protocol) carries metadata about the media session — packet loss reports, bandwidth estimates, keyframe requests ([section 10](#10-the-rtcp-drain-goroutine))
- **DTLS** (Datagram Transport Layer Security) negotiates encryption keys over UDP
- **ICE** (Interactive Connectivity Establishment) discovers network paths between peers ([section 9](#9-ice-nat-traversal-with-stun-and-turn))
- **SDP** (Session Description Protocol) describes what media each side wants to send/receive ([section 3](#3-the-signaling-layer-sdp-and-ice))

A browser's `RTCPeerConnection` API orchestrates all of this. You create one, add media tracks, exchange [SDP offers/answers](#3-the-signaling-layer-sdp-and-ice) with the other side, and the browser handles encryption, codec negotiation, congestion control, jitter buffering, and packet retransmission automatically.

The key thing WebRTC does **not** provide is signaling: there's no built-in way to exchange SDP and ICE candidates between peers. You need your own transport for that, which is where the existing WebSocket connection comes in ([section 14](#14-signaling-over-an-existing-websocket)).

---

## 2. SFU Architecture vs. Mesh vs. MCU

There are three common architectures for multi-party WebRTC calls:

**Mesh** (peer-to-peer): Every participant connects directly to every other participant. For N users, each uploads their stream N-1 times. This doesn't scale past ~4 people because upload bandwidth grows linearly.

**MCU** (Multipoint Control Unit): A central server receives all streams, decodes them, mixes them into a single composite stream, re-encodes, and sends one stream to each participant. CPU-intensive because of the decode/encode step.

**SFU** (Selective Forwarding Unit): A central server receives each participant's stream and **forwards the raw encrypted packets** to every other participant without decoding or re-encoding ([section 6](#6-rtp-forwarding-the-core-of-the-sfu)). Each participant uploads once (to the server) and downloads N-1 streams. The server's CPU cost is near-zero because it just copies bytes.

We chose SFU because:
- **Minimal CPU**: No transcoding. Opus audio packets arrive and get forwarded as-is.
- **Low latency**: No decode/encode round-trip on the server.
- **Scales well**: Each participant has exactly one [PeerConnection](#4-pion-webrtc-a-go-native-webrtc-stack) to the server.
- **Codec agnostic**: The server doesn't care what codec the clients negotiate.

**Crucially, clients never connect to each other.** Every client's `RTCPeerConnection` is a connection to the server — the Go server running Pion *is* the "peer" on the other end. From the browser's perspective, it looks like a normal WebRTC call with a single peer that just happens to send back multiple audio tracks from different people. The browser has no idea it's talking to a forwarding server rather than another user.

```
    User A ←── PeerConnection ──→  Server  ←── PeerConnection ──→ User B
                                     ↕
                               PeerConnection
                                     ↕
                                   User C
```

A uploads audio once to the server. The server's [forwarding goroutines](#6-rtp-forwarding-the-core-of-the-sfu) copy those RTP packets into B's and C's PeerConnections. B and C each receive A's track as if A were directly connected to them.

The tradeoff is that each client downloads N-1 streams instead of 1 mixed stream. For voice chat with typical group sizes (2-20 people), Opus at ~32kbps per stream means 20 participants = ~600kbps download, which is trivial for any modern connection.

---

## 3. The Signaling Layer: SDP and ICE

Before any media flows, two sides must agree on what they're sending. This happens via **SDP** (Session Description Protocol) — a text format that describes:

- What media types are offered (audio, video)
- What codecs are supported (Opus, VP8, H.264, etc.)
- ICE credentials (username fragment and password)
- DTLS fingerprint (for encryption verification)
- Media directions (sendrecv, recvonly, sendonly)

The exchange follows the **offer/answer model**:

1. The **offerer** creates an SDP offer describing what it wants to send/receive
2. The **answerer** receives the offer, creates a compatible SDP answer
3. Both sides set their local and remote descriptions

In our implementation, the client is the initial offerer:

```
Client                          Server
  |                               |
  |-- voice_offer (SDP offer) --> |  Client creates offer, sends over WS
  |                               |  Server calls SetRemoteDescription(offer)
  |                               |  Server calls CreateAnswer()
  |                               |  Server calls SetLocalDescription(answer)
  |<-- voice_answer (SDP answer) -|  Server sends answer back over WS
  |                               |
  |  Client calls setRemoteDescription(answer)
  |                               |
  |== DTLS handshake, media flows ==|
```

But when the server needs to add new tracks (a new participant joined), the **server becomes the offerer** — this is called [renegotiation](#8-server-initiated-renegotiation).

---

## 4. Pion WebRTC: A Go-Native WebRTC Stack

[Pion](https://github.com/pion/webrtc) is a pure Go implementation of the entire WebRTC stack. It doesn't wrap a C++ library (like libwebrtc) — it implements [ICE](#9-ice-nat-traversal-with-stun-and-turn), DTLS, SRTP, SCTP, and RTP entirely in Go. This means:

- No CGo, no cross-compilation headaches
- Full control over every layer (you can intercept RTP packets, write custom ICE logic, etc.)
- The `webrtc.PeerConnection` API mirrors the browser's `RTCPeerConnection`

Key Pion types used in our SFU:

| Type | Purpose |
|------|---------|
| `webrtc.PeerConnection` | One per participant. Handles [SDP](#3-the-signaling-layer-sdp-and-ice), [ICE](#9-ice-nat-traversal-with-stun-and-turn), DTLS, media tracks. |
| `webrtc.TrackRemote` | A track the server *receives* from a client (e.g., their microphone audio). Used in [RTP forwarding](#6-rtp-forwarding-the-core-of-the-sfu). |
| `webrtc.TrackLocalStaticRTP` | A track the server *sends* to a client. We write raw RTP packets to it. Created via the [StreamID trick](#7-track-identity-the-streamid-trick). |
| `webrtc.ICECandidateInit` | An ICE candidate received from the client for [trickle ICE](#15-ice-candidate-buffering-trickle-ice). |
| `webrtc.SessionDescription` | An [SDP](#3-the-signaling-layer-sdp-and-ice) offer or answer. |
| `rtcp.PictureLossIndication` | An RTCP message requesting a video [keyframe](#11-pli-getting-keyframes-for-video). |

---

## 5. Server-Side Architecture

The SFU is organized into four files under `src/internal/voice/`:

### Manager (`manager.go`)
The top-level coordinator. Maps channel UUIDs to Room instances. Routes [signaling messages](#14-signaling-over-an-existing-websocket) (`HandleOffer`, `HandleAnswer`, `HandleICECandidate`) to the correct Participant in the correct Room. Creates rooms on demand when the first user joins, deletes them when the last user leaves ([lifecycle details](#18-lifecycle-and-cleanup)).

### Room (`room.go`)
Represents one voice channel. Holds a map of `uuid -> *Participant`. When a participant publishes a track, the Room creates a corresponding `TrackLocalStaticRTP` for every *other* participant and starts an [RTP forwarding goroutine](#6-rtp-forwarding-the-core-of-the-sfu). When a participant leaves, the Room removes their tracks from everyone else and triggers [renegotiation](#8-server-initiated-renegotiation).

### Participant (`participant.go`)
Wraps a single `webrtc.PeerConnection` for one user. Handles:
- [SDP offer/answer](#3-the-signaling-layer-sdp-and-ice) exchange
- [ICE candidate](#15-ice-candidate-buffering-trickle-ice) addition
- Track subscription (adding other people's tracks to this user's PeerConnection)
- The `OnTrack` callback (when the client publishes audio/video)
- [Server-initiated renegotiation](#8-server-initiated-renegotiation) (creating offers when tracks change)

### Track (`track.go`)
Defines `ForwardedTrack` (metadata about a published track) and the `newLocalTrackFromRemote` helper that creates a `TrackLocalStaticRTP` matching the codec of a `TrackRemote`, with the [StreamID encoding the publisher's user ID](#7-track-identity-the-streamid-trick).

---

## 6. RTP Forwarding: The Core of the SFU

This is the heart of the entire system. When User A publishes their microphone:

1. Pion calls `participant.onTrack(remoteTrack, receiver)` on A's PeerConnection
2. The Room creates a `TrackLocalStaticRTP` for each other participant (B, C, D...)
3. Each local track is added to the respective participant's PeerConnection via `AddTrack()`
4. A goroutine starts reading RTP packets from `remoteTrack` and writing them to all local tracks

The forwarding goroutine is strikingly simple:

```go
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
            return
        }

        for _, localTrack := range localTracks {
            localTrack.Write(buf[:readCount])
        }
    }
}
```

That's it. `remote.Read(buf)` blocks until an RTP packet arrives from the client, then `localTrack.Write(buf[:readCount])` sends the exact same bytes to each subscriber. The packet is already encrypted via SRTP between each [PeerConnection](#4-pion-webrtc-a-go-native-webrtc-stack) — Pion handles the decryption on receive and re-encryption on send transparently. The audio content (Opus frames) is never decoded; it's pure byte forwarding.

The 1500-byte buffer matches the typical MTU (Maximum Transmission Unit) of Ethernet networks, which is the maximum size of a single UDP packet. Opus audio frames at 32kbps produce packets well under this limit.

The `stop` channel allows the Room to kill the forwarding goroutine when a participant leaves — it's the standard Go pattern for cancellable goroutines using a `chan struct{}`.

---

## 7. Track Identity: The StreamID Trick

A critical problem in any SFU: when the client receives a remote track via the `ontrack` event, how does it know *which user* the track belongs to?

Some SFUs solve this with out-of-band metadata messages. We use a simpler trick: **encode the user's identity in the RTP stream ID**.

On the server, when creating a local track to forward to subscribers:

```go
func newLocalTrackFromRemote(remote *webrtc.TrackRemote, userID uuid.UUID, source string) (*webrtc.TrackLocalStaticRTP, error) {
    return webrtc.NewTrackLocalStaticRTP(
        remote.Codec().RTPCodecCapability,
        remote.ID(),
        streamID(userID, source),  // e.g. "a1b2c3d4-...:microphone"
    )
}
```

The third parameter to `NewTrackLocalStaticRTP` is the **stream ID**. In the [SDP](#3-the-signaling-layer-sdp-and-ice), this becomes the `msid` attribute. When the browser fires `RTCTrackEvent`, the stream ID is available as `event.streams[0].id`.

On the client:

```typescript
peerConnection.ontrack = (event) => {
    const streamId = event.streams[0].id;       // "a1b2c3d4-...:microphone"
    const colonIndex = streamId.indexOf(':');
    const userId = streamId.substring(0, colonIndex);   // "a1b2c3d4-..."
    const source = streamId.substring(colonIndex + 1);  // "microphone"
};
```

This way, the client immediately knows which user and which source (microphone, screen video, screen audio) each track belongs to — with zero extra signaling messages.

---

## 8. Server-Initiated Renegotiation

The initial [offer/answer exchange](#3-the-signaling-layer-sdp-and-ice) only describes the tracks that exist at connection time. But in a voice chat, participants join and leave dynamically. When User B joins a room where User A is already talking, the server needs to add A's audio track to B's PeerConnection.

This is done via **server-initiated renegotiation**: the server creates a new [SDP](#3-the-signaling-layer-sdp-and-ice) offer and sends it to the client via the [WebSocket signaling channel](#14-signaling-over-an-existing-websocket).

In `participant.go`, the `negotiate()` method:

```go
func (participant *Participant) negotiate() error {
    offer, _ := participant.peerConn.CreateOffer(nil)
    participant.peerConn.SetLocalDescription(offer)

    // Send the offer to the client over WebSocket
    data, _ := json.Marshal(map[string]any{
        "type":       "voice_offer",
        "channel_id": participant.Room.ChannelID.String(),
        "sdp":        map[string]string{"type": "offer", "sdp": offer.SDP},
    })
    participant.sendMessage(data)
    return nil
}
```

This is called automatically whenever `AddSubscription()` or `RemoveSubscription()` modifies the tracks on a PeerConnection. The client handles it:

```typescript
function handleVoiceOffer(data) {
    const offer = new RTCSessionDescription({ type: data.sdp.type, sdp: data.sdp.sdp });
    peerConnection.setRemoteDescription(offer)
        .then(() => peerConnection.createAnswer())
        .then((answer) => peerConnection.setLocalDescription(answer))
        .then(() => {
            websocket.send({
                type: 'voice_answer',
                channel_id: channelId,
                sdp: { type: peerConnection.localDescription.type, sdp: peerConnection.localDescription.sdp },
            });
        });
}
```

The flow is the same [offer/answer pattern](#3-the-signaling-layer-sdp-and-ice), just with reversed roles. The SDP now contains additional `m=` lines for the new tracks. The browser automatically starts receiving those tracks and fires `ontrack` for each new one (where the [StreamID trick](#7-track-identity-the-streamid-trick) identifies whose audio it is).

---

## 9. ICE: NAT Traversal with STUN and TURN

Most computers sit behind a NAT (Network Address Translation) router that assigns private IP addresses (192.168.x.x). For WebRTC to work, both sides need to discover their public IP:port combinations. This is what ICE does.

**STUN** (Session Traversal Utilities for NAT): The client sends a UDP packet to a STUN server (e.g., `stun:stun.l.google.com:19302`). The server replies with the client's public IP and port as seen from the outside. This is called a **server-reflexive candidate**. Works for most NAT types.

**TURN** (Traversal Using Relays around NAT): When STUN fails (symmetric NAT), TURN acts as a relay. All media flows through the TURN server. More reliable but adds latency and bandwidth cost. The TURN server must be explicitly configured with credentials.

ICE gathers multiple candidates (host candidates from local interfaces, server-reflexive from STUN, relay from TURN) and tries them in priority order. The process is called **ICE candidate gathering**.

In our implementation, ICE servers are configured per-environment:

```go
// Server side
voiceManager = voice.NewManager([]webrtc.ICEServer{
    {URLs: []string{"stun:stun.l.google.com:19302"}},
})
```

```typescript
// Client side (from /api/config response)
new RTCPeerConnection({ iceServers: configStore.iceServers });
```

Both the server (Pion) and the client (browser) need the same ICE configuration so they can gather compatible candidates.

---

## 10. The RTCP Drain Goroutine

**RTCP** (RTP Control Protocol) is the companion protocol to RTP. While RTP carries the actual media (audio/video packets), RTCP carries **metadata about the media session** — it runs on the same connection but serves a completely different purpose. RTCP messages include:

- **Receiver Reports (RR)**: "I received 98% of your packets in the last interval" — lets the sender know about packet loss
- **Sender Reports (SR)**: "I sent X packets at Y bitrate" — synchronization and statistics
- **NACK**: "I'm missing packet #4527, please resend it" — selective retransmission requests
- **PLI** (Picture Loss Indication): "I need a keyframe" — covered in [section 11](#11-pli-getting-keyframes-for-video)
- **REMB/TWcc**: Bandwidth estimation feedback for congestion control

RTCP is how WebRTC peers communicate *about* the media without interrupting the media itself. Both sides send RTCP periodically (typically every few seconds) regardless of whether they're sending media.

When you call `PeerConnection.AddTrack()` in [Pion](#4-pion-webrtc-a-go-native-webrtc-stack), it returns an `*RTPSender`. This sender has an internal RTCP receive buffer. The remote peer periodically sends RTCP packets back through this sender.

**If you don't read from this buffer, it fills up and blocks.** This is a common [Pion](#4-pion-webrtc-a-go-native-webrtc-stack) gotcha. The solution is a drain goroutine:

```go
sender, _ := participant.peerConn.AddTrack(localTrack)

go func() {
    rtcpBuf := make([]byte, 1500)
    for {
        if _, _, readErr := sender.Read(rtcpBuf); readErr != nil {
            return
        }
    }
}()
```

This goroutine reads and discards RTCP packets forever. In a more sophisticated SFU, you'd parse these RTCP packets to implement:
- **NACK handling**: Retransmit lost packets
- **REMB/TWcc**: Bandwidth estimation for congestion control
- **Receiver reports**: Monitor packet loss statistics

For a voice-focused SFU where Opus handles packet loss gracefully, draining is sufficient.

---

## 11. PLI: Getting Keyframes for Video

Video codecs (VP8, H.264) use two types of frames:
- **Keyframes** (I-frames): Complete picture, can be decoded independently
- **Delta frames** (P-frames): Only contain changes from the previous frame

When a new subscriber joins mid-stream, they haven't received the initial keyframe, so they can't decode any delta frames. The video will appear as garbled noise or simply not render.

The fix is **PLI** (Picture Loss Indication) — an RTCP message sent from the SFU to the publisher requesting a new keyframe:

```go
if remote.Kind() == webrtc.RTPCodecTypeVideo {
    go func() {
        ticker := time.NewTicker(3 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            participant.peerConn.WriteRTCP([]rtcp.Packet{
                &rtcp.PictureLossIndication{MediaSSRC: uint32(remote.SSRC())},
            })
        }
    }()
}
```

The `SSRC` (Synchronization Source) identifies which specific media stream we want a keyframe for. The publisher's browser receives this RTCP PLI and immediately encodes and sends a keyframe. The 3-second interval ensures new joiners see [screen share](#17-screen-sharing-via-addtrack) content quickly.

For audio, PLI doesn't apply — Opus is a stateless codec where every packet can be decoded independently.

---

## 12. Client-Side Audio Processing Pipeline

Before the microphone audio reaches the [PeerConnection](#4-pion-webrtc-a-go-native-webrtc-stack), it passes through a processing pipeline built on the Web Audio API:

```
Raw Mic Track → [RNNoise WASM] → [Noise Gate] → Processed Track → PeerConnection
```

### RNNoise (Neural Noise Suppression)

[RNNoise](https://jmvalin.ca/demo/rnnoise/) is a neural network-based noise suppressor. It runs as a WebAssembly module inside an AudioWorklet:

```typescript
// Load WASM binary (cached after first load)
const wasmBinary = await loadRnnoise({ url: rnnoiseWasmPath, simdUrl: rnnoiseSimdWasmPath });

// Register the AudioWorklet processor
await audioContext.audioWorklet.addModule(rnnoiseWorkletPath);

// Build pipeline: source → rnnoise → destination
const sourceNode = audioContext.createMediaStreamSource(inputStream);
const rnnoiseNode = new RnnoiseWorkletNode(audioContext, { wasmBinary, maxChannels: 1 });
const destinationNode = audioContext.createMediaStreamDestination();

sourceNode.connect(rnnoiseNode);
rnnoiseNode.connect(destinationNode);

// The output track goes to the PeerConnection
const processedTrack = destinationNode.stream.getAudioTracks()[0];
```

The AudioWorklet runs on a separate thread (the audio rendering thread), so it doesn't block the main thread. The WASM module processes 480 samples at a time (10ms of 48kHz audio), applying a trained RNN model that separates speech from noise.

### Noise Gate

The noise gate is a threshold-based mute. It monitors the RMS (Root Mean Square) level of the audio and mutes the output when it drops below a configurable threshold:

```
source → analyser → gain → destination
```

The `AnalyserNode` computes time-domain data. Every 50ms, the gate reads the RMS level, compares it to the threshold, and smoothly ramps the `GainNode` between 0 (muted) and 1 (unmuted) using `setTargetAtTime()` with a 15ms time constant to avoid audio clicks.

The gate also reports speaking state (`onGateStateChange`) and mic level (`onMicLevelChange`) back to the UI for the [speaking indicators](#16-mute-and-speaking-state-without-a-media-server) and level meter.

### Applying Processing Before the PeerConnection

Previously with LiveKit, audio processing used LiveKit's `TrackProcessor` interface which intercepted audio inside the SDK. With native WebRTC, we process the track **before** adding it:

```typescript
// Get raw microphone
const localStream = await navigator.mediaDevices.getUserMedia({ audio: { ... } });
const rawTrack = localStream.getAudioTracks()[0];

// Process it
const result = await createAudioProcessor(rawTrack, audioContext, options);
const trackToSend = result?.processedTrack ?? rawTrack;

// Add the processed track to the PeerConnection
peerConnection.addTrack(trackToSend, new MediaStream([trackToSend]));
```

The browser negotiates Opus encoding for whatever track we give it. It doesn't matter whether it's the raw mic or a processed version.

---

## 13. Mono-to-Stereo Upmixing for Echo Cancellation

Opus typically encodes voice as mono (1 channel). Browser echo cancellation algorithms (enabled via [`getUserMedia` constraints](#12-client-side-audio-processing-pipeline)) work best with a stereo reference signal — they need to match the signal coming out of the speakers with what the microphone picks up.

A mono stream played through stereo speakers can confuse the echo canceller because the acoustic path differs per ear. The fix is to **upmix mono to stereo** via the Web Audio API before playback:

```typescript
const source = audioContext.createMediaStreamSource(stream);
const splitter = audioContext.createChannelSplitter(1);    // Split mono into 1 channel
const merger = audioContext.createChannelMerger(2);         // Merge into 2 channels

source.connect(splitter);
splitter.connect(merger, 0, 0);  // Mono → Left channel
splitter.connect(merger, 0, 1);  // Mono → Right channel (duplicate)
merger.connect(streamDestination);

audioElement.srcObject = streamDestination.stream;  // Play the stereo version
```

The `ChannelSplitter` takes the single mono channel and outputs it. The `ChannelMerger` takes two inputs and combines them into a stereo stream. By connecting the same mono signal to both input 0 (left) and input 1 (right), we get identical content on both channels — a proper stereo signal that the echo canceller can work with.

We also use JavaScript `Symbol` keys to store Web Audio node references on the `<audio>` element without risking name collisions with browser-internal properties:

```typescript
const SOURCE_NODE_KEY = Symbol('voiceSourceNode');
audioElement[SOURCE_NODE_KEY] = source;
```

---

## 14. Signaling Over an Existing WebSocket

Most WebRTC applications use a dedicated signaling server. We reuse the application's existing WebSocket connection (used for chat messages, presence, typing indicators) for WebRTC signaling — carrying [SDP offers/answers](#3-the-signaling-layer-sdp-and-ice) and [ICE candidates](#9-ice-nat-traversal-with-stun-and-turn). This eliminates an extra connection and simplifies authentication.

New message types added to the existing WS protocol:

| Message | Direction | Purpose |
|---------|-----------|---------|
| `voice_join` | client -> server | Notify hub to track voice state, create PeerConnection |
| `voice_leave` | client -> server | Leave voice channel, tear down PeerConnection |
| `voice_offer` | bidirectional | SDP offer (client initial, server renegotiation) |
| `voice_answer` | bidirectional | SDP answer (server initial, client renegotiation) |
| `voice_ice_candidate` | bidirectional | Trickle ICE candidates |
| `voice_mute_state` | bidirectional | Mute/unmute notifications |
| `voice_speaking` | bidirectional | Speaking state from noise gate |

The `maxMessageSize` was increased from 4096 to 65536 bytes because [SDP](#3-the-signaling-layer-sdp-and-ice) offers with multiple media sections (one per track due to [renegotiation](#8-server-initiated-renegotiation)) can easily exceed 4KB.

Voice signaling messages (`voice_offer`, `voice_answer`, `voice_ice_candidate`) are handled directly in the client's read goroutine — they call into the voice [Manager](#5-server-side-architecture) which uses its own mutexes. This is safe because [Pion's](#4-pion-webrtc-a-go-native-webrtc-stack) `PeerConnection` is goroutine-safe. State messages (`voice_mute_state`, `voice_speaking`) go through the Hub's channel-based event loop for broadcast to all channel subscribers.

---

## 15. ICE Candidate Buffering (Trickle ICE)

ICE candidates are gathered asynchronously. A candidate might arrive from the server *before* the client has finished setting the remote description (the SDP answer). If you call `addIceCandidate()` before `setRemoteDescription()`, the browser throws an error.

The solution is **candidate buffering**:

```typescript
let pendingIceCandidates: RTCIceCandidateInit[] = [];
let remoteDescriptionSet = false;

function handleVoiceIceCandidate(data) {
    if (!remoteDescriptionSet) {
        pendingIceCandidates.push(data.candidate);  // Buffer it
        return;
    }
    peerConnection.addIceCandidate(new RTCIceCandidate(data.candidate));
}

function handleVoiceAnswer(data) {
    peerConnection.setRemoteDescription(answer).then(() => {
        remoteDescriptionSet = true;
        // Flush buffered candidates
        for (const candidate of pendingIceCandidates) {
            peerConnection.addIceCandidate(new RTCIceCandidate(candidate));
        }
        pendingIceCandidates = [];
    });
}
```

This is called **trickle ICE** — candidates trickle in as they're discovered, rather than waiting for all candidates to be gathered before exchanging [SDP](#3-the-signaling-layer-sdp-and-ice). It significantly reduces connection setup time because [STUN](#9-ice-nat-traversal-with-stun-and-turn) requests and ICE connectivity checks can begin while SDP is still being exchanged.

---

## 16. Mute and Speaking State Without a Media Server

With LiveKit, mute state and active speaker detection were handled server-side. Without a media server inspecting audio levels, we use two approaches:

### Mute

Track-level disable: `sender.track.enabled = false` causes the browser to send silence (empty Opus frames) instead of actual audio. The [RTP stream](#6-rtp-forwarding-the-core-of-the-sfu) continues but carries no audible content. A `voice_mute_state` [WebSocket message](#14-signaling-over-an-existing-websocket) broadcasts the mute state to all participants in the channel.

### Speaking Detection

**Local user**: The [noise gate's](#12-client-side-audio-processing-pipeline) `onGateStateChange` callback fires when the gate opens (voice detected) or closes (silence). This drives the local speaking indicator and broadcasts `voice_speaking` over [WebSocket](#14-signaling-over-an-existing-websocket).

**Remote users**: Each client sends `voice_speaking` messages when their local noise gate state changes. Other clients receive these and update `speakingUserIds`. This is pure application-level [signaling](#14-signaling-over-an-existing-websocket) — the server just broadcasts the message to the channel.

This approach is simpler than server-side audio level detection (which would require parsing RTP packet headers for audio levels or decoding audio). The latency is slightly higher (a round-trip through the WS) but imperceptible for a speaking indicator.

---

## 17. Screen Sharing via addTrack

Screen sharing uses the browser's `getDisplayMedia()` API to capture a screen or window:

```typescript
const stream = await navigator.mediaDevices.getDisplayMedia({
    video: { width: { ideal: 1920 }, height: { ideal: 1080 }, frameRate: { ideal: 30 } },
    audio: true,
});
```

The returned `MediaStream` contains a video track (the screen content) and optionally an audio track (system audio). These are added to the **existing PeerConnection**:

```typescript
const videoTrack = stream.getVideoTracks()[0];
const sender = peerConnection.addTrack(videoTrack, new MediaStream([videoTrack]));
screenShareSenders.push(sender);
```

Adding a track triggers [Pion's](#4-pion-webrtc-a-go-native-webrtc-stack) `OnTrack` callback on the server. The server classifies the track based on its kind (video = "screen", audio with "screen"/"display" in stream ID = "screen_audio"), and [forwards it](#6-rtp-forwarding-the-core-of-the-sfu) to all other participants just like voice audio. For video tracks, the server also starts sending [PLI requests](#11-pli-getting-keyframes-for-video) to ensure new viewers get keyframes.

To stop sharing, we remove the senders:

```typescript
for (const sender of screenShareSenders) {
    sender.track?.stop();            // Stops capture
    peerConnection.removeTrack(sender);  // Removes from SDP
}
```

This triggers [renegotiation](#8-server-initiated-renegotiation) — the server sends a new offer without those tracks, and other clients' `ontrack` listeners fire `track.onended`.

---

## 18. Lifecycle and Cleanup

Voice state has two parallel tracking systems:

1. **Hub's `voiceUsers` map** — tracks which users are in which channels, broadcasts `voice_state_update` messages to all clients for the UI (participant lists, join/leave sounds)
2. **Voice Manager's rooms** — tracks actual PeerConnections and RTP forwarding

Both are updated together (via the [signaling layer](#14-signaling-over-an-existing-websocket)):
- `voice_join` → Hub adds to `voiceUsers` + [Manager](#5-server-side-architecture) creates PeerConnection
- `voice_leave` → Hub removes from `voiceUsers` + Manager tears down PeerConnection
- WebSocket disconnect → Hub detects last connection gone → Manager calls `LeaveAllRooms()`

On the client side, cleanup is thorough:
- `PeerConnection.close()` stops all ICE agents and DTLS sessions
- `localStream.getTracks().forEach(track => track.stop())` releases the microphone
- Remote audio `<audio>` elements are removed from the DOM
- Web Audio nodes are disconnected to prevent memory leaks
- The shared `AudioContext` is closed

The `AbortController` pattern allows `leave()` to instantly cancel any pending connection retry loop:

```typescript
connectionAbortController?.abort();  // Cancels any pending retry
connectionAbortController = null;
```

---

## 19. Technologies Used

| Technology | Role | Covered in |
|---|---|---|
| **Pion WebRTC v4** (`github.com/pion/webrtc/v4`) | Go-native WebRTC stack: PeerConnection, ICE, DTLS, SRTP | [Section 4](#4-pion-webrtc-a-go-native-webrtc-stack) |
| **Pion RTP** (`github.com/pion/rtp`) | RTP packet reading/writing for media forwarding | [Section 6](#6-rtp-forwarding-the-core-of-the-sfu) |
| **Pion RTCP** (`github.com/pion/rtcp`) | RTCP messages (PLI for keyframe requests, receiver reports) | [Section 10](#10-the-rtcp-drain-goroutine), [Section 11](#11-pli-getting-keyframes-for-video) |
| **RTCPeerConnection** (Browser API) | Client-side WebRTC: SDP negotiation, ICE, media tracks | [Section 1](#1-what-is-webrtc), [Section 2](#2-sfu-architecture-vs-mesh-vs-mcu) |
| **Web Audio API** (Browser API) | Audio processing pipeline: noise gate, stereo upmixing | [Section 12](#12-client-side-audio-processing-pipeline), [Section 13](#13-mono-to-stereo-upmixing-for-echo-cancellation) |
| **AudioWorklet** (Browser API) | Runs RNNoise WASM on a dedicated audio thread | [Section 12](#12-client-side-audio-processing-pipeline) |
| **WebAssembly (WASM)** | RNNoise neural noise suppression compiled from C to WASM | [Section 12](#12-client-side-audio-processing-pipeline) |
| **Opus** | Audio codec negotiated by WebRTC. 32-64kbps, handles packet loss gracefully | [Section 6](#6-rtp-forwarding-the-core-of-the-sfu) |
| **SRTP** | Encrypts RTP media packets. Negotiated via DTLS. Transparent to the SFU. | [Section 1](#1-what-is-webrtc), [Section 6](#6-rtp-forwarding-the-core-of-the-sfu) |
| **DTLS** | Key exchange for SRTP. Runs over the same UDP connection as media. | [Section 1](#1-what-is-webrtc) |
| **ICE / STUN / TURN** | NAT traversal. STUN discovers public IP; TURN relays when STUN fails. | [Section 9](#9-ice-nat-traversal-with-stun-and-turn) |
| **SDP** | Describes media sessions (codecs, tracks, ICE credentials). Text format. | [Section 3](#3-the-signaling-layer-sdp-and-ice) |
| **gorilla/websocket** | Go WebSocket library used for existing app WS (signaling reuses this) | [Section 14](#14-signaling-over-an-existing-websocket) |
| **Svelte 5 runes** | Reactive UI framework. `$state` and `$derived` for voice state management. | — |
| **`getDisplayMedia()`** (Browser API) | Screen/window capture for screen sharing | [Section 17](#17-screen-sharing-via-addtrack) |
| **`getUserMedia()`** (Browser API) | Microphone capture with echo cancellation constraints | [Section 12](#12-client-side-audio-processing-pipeline) |
| **`@sapphi-red/web-noise-suppressor`** | npm package wrapping RNNoise WASM with AudioWorklet integration | [Section 12](#12-client-side-audio-processing-pipeline) |
