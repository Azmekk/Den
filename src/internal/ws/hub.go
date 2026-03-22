package ws

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/voice"
)

type MessageHandler interface {
	SendMessage(ctx context.Context, channelID, userID uuid.UUID, username, content string, replyToID *uuid.UUID) ([]byte, []uuid.UUID, error)
	EditMessage(ctx context.Context, messageID, userID uuid.UUID, content string) ([]byte, uuid.UUID, uuid.UUID, error)
	DeleteMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) (uuid.UUID, uuid.UUID, error)
}

type DMMessageHandler interface {
	SendDMMessage(ctx context.Context, dmPairID, userID uuid.UUID, username, content string, replyToID *uuid.UUID) ([]byte, []uuid.UUID, error)
	ValidateUserInPair(ctx context.Context, dmPairID, userID uuid.UUID) (uuid.UUID, error)
}

type subRequest struct {
	client    *Client
	channelID uuid.UUID
}

type directMsg struct {
	client *Client
	data   []byte
}

type broadcastExcludeMsg struct {
	channelID uuid.UUID
	data      []byte
	exclude   *Client
}

type userMsg struct {
	userID uuid.UUID
	data   []byte
}

type voiceAction struct {
	client    *Client
	channelID uuid.UUID
}

// VoiceUserState tracks per-user state within a voice channel.
type VoiceUserState struct {
	Muted     bool `json:"muted"`
	Deafened  bool `json:"deafened"`
	Streaming bool `json:"streaming"`
}

type voiceStateChange struct {
	userID    uuid.UUID
	muted     *bool
	deafened  *bool
	streaming *bool
}

type voiceBroadcastMsg struct {
	channelID uuid.UUID
	data      []byte
}

type Hub struct {
	clients         map[*Client]bool
	channels        map[uuid.UUID]map[*Client]bool
	onlineUsers     map[uuid.UUID]map[*Client]bool
	voiceUsers      map[uuid.UUID]map[uuid.UUID]*VoiceUserState // channelID → userID → state
	register        chan *Client
	unregister      chan *Client
	subscribe       chan subRequest
	unsub           chan subRequest
	broadcast       chan broadcastMsg
	directSend      chan directMsg
	broadcastExc    chan broadcastExcludeMsg
	globalBroadcast chan []byte
	userSend        chan userMsg
	voiceJoin       chan voiceAction
	voiceLeave      chan voiceAction
	voiceState      chan voiceStateChange
	voiceBroadcast  chan voiceBroadcastMsg
	kickUser        chan uuid.UUID
	VoiceManager    *voice.Manager
}

type broadcastMsg struct {
	channelID uuid.UUID
	data      []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		channels:        make(map[uuid.UUID]map[*Client]bool),
		onlineUsers:     make(map[uuid.UUID]map[*Client]bool),
		voiceUsers:      make(map[uuid.UUID]map[uuid.UUID]*VoiceUserState),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		subscribe:       make(chan subRequest),
		unsub:           make(chan subRequest),
		broadcast:       make(chan broadcastMsg, 256),
		directSend:      make(chan directMsg, 256),
		broadcastExc:    make(chan broadcastExcludeMsg, 256),
		globalBroadcast: make(chan []byte, 256),
		userSend:        make(chan userMsg, 256),
		voiceJoin:       make(chan voiceAction),
		voiceLeave:      make(chan voiceAction),
		voiceState:      make(chan voiceStateChange),
		voiceBroadcast:  make(chan voiceBroadcastMsg, 256),
		kickUser:        make(chan uuid.UUID),
	}
}

func (h *Hub) broadcastAll(data []byte) {
	var overflow []*Client
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			overflow = append(overflow, client)
		}
	}
	for _, client := range overflow {
		userID := client.UserID
		username := client.Username
		wasLast := h.removeClient(client)
		if wasLast {
			update, _ := json.Marshal(map[string]any{
				"type":     "presence_update",
				"user_id":  userID,
				"username": username,
				"status":   "offline",
			})
			h.broadcastAll(update)
		}
	}
}

// removeClient removes a client and returns true if it was the user's last connection.
func (h *Hub) removeClient(client *Client) bool {
	delete(h.clients, client)
	close(client.send)
	client.subsMu.Lock()
	defer client.subsMu.Unlock()
	for chID := range client.subs {
		if m, ok := h.channels[chID]; ok {
			delete(m, client)
			if len(m) == 0 {
				delete(h.channels, chID)
			}
		}
	}
	if conns, ok := h.onlineUsers[client.UserID]; ok {
		delete(conns, client)
		if len(conns) == 0 {
			delete(h.onlineUsers, client.UserID)
			h.removeUserFromVoice(client.UserID)
			return true
		}
	}
	return false
}

func (h *Hub) removeUserFromVoice(userID uuid.UUID) {
	for channelID, users := range h.voiceUsers {
		if _, ok := users[userID]; ok {
			if h.VoiceManager != nil {
				h.VoiceManager.LeaveRoom(channelID, userID)
			}
			delete(users, userID)
			if len(users) == 0 {
				delete(h.voiceUsers, channelID)
			}
			h.broadcastVoiceState()
			return
		}
	}
}

func (h *Hub) removeUserFromVoiceNoNotify(userID uuid.UUID) {
	for channelID, users := range h.voiceUsers {
		if _, ok := users[userID]; ok {
			delete(users, userID)
			if len(users) == 0 {
				delete(h.voiceUsers, channelID)
			}
			return
		}
	}
}

func (h *Hub) broadcastVoiceState() {
	state := h.buildVoiceStates()
	data, _ := json.Marshal(map[string]any{
		"type":         "voice_state_update",
		"voice_states": state,
	})
	h.broadcastAll(data)
}

func (h *Hub) buildVoiceStates() map[string][]map[string]any {
	result := make(map[string][]map[string]any)
	for channelID, users := range h.voiceUsers {
		participants := make([]map[string]any, 0, len(users))
		for userID, state := range users {
			participants = append(participants, map[string]any{
				"user_id":   userID.String(),
				"muted":     state.Muted,
				"deafened":  state.Deafened,
				"streaming": state.Streaming,
			})
		}
		result[channelID.String()] = participants
	}
	return result
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

			// Track online users
			if _, ok := h.onlineUsers[client.UserID]; !ok {
				h.onlineUsers[client.UserID] = make(map[*Client]bool)
			}
			isFirstConnection := len(h.onlineUsers[client.UserID]) == 0
			h.onlineUsers[client.UserID][client] = true

			// Send presence_initial to newly connected client
			onlineIDs := make([]uuid.UUID, 0, len(h.onlineUsers))
			for uid := range h.onlineUsers {
				onlineIDs = append(onlineIDs, uid)
			}
			initMsg, _ := json.Marshal(map[string]any{
				"type":            "presence_initial",
				"online_user_ids": onlineIDs,
			})
			select {
			case client.send <- initMsg:
			default:
			}

			// Send voice_state_initial
			voiceInitMsg, _ := json.Marshal(map[string]any{
				"type":         "voice_state_initial",
				"voice_states": h.buildVoiceStates(),
			})
			select {
			case client.send <- voiceInitMsg:
			default:
			}

			// Broadcast user_registered if this is a brand new user
			if client.IsNewUser {
				registeredMsg, _ := json.Marshal(map[string]any{
					"type":         "user_registered",
					"id":           client.UserID,
					"username":     client.Username,
					"display_name": client.DisplayName,
					"is_admin":     client.IsAdmin,
				})
				h.broadcastAll(registeredMsg)
			}

			// Broadcast online status if first connection
			if isFirstConnection {
				update, _ := json.Marshal(map[string]any{
					"type":     "presence_update",
					"user_id":  client.UserID,
					"username": client.Username,
					"status":   "online",
				})
				h.broadcastAll(update)
			}

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				userID := client.UserID
				username := client.Username
				wasLast := h.removeClient(client)

				if wasLast {
					update, _ := json.Marshal(map[string]any{
						"type":     "presence_update",
						"user_id":  userID,
						"username": username,
						"status":   "offline",
					})
					h.broadcastAll(update)
				}
			}

		case req := <-h.subscribe:
			if _, ok := h.channels[req.channelID]; !ok {
				h.channels[req.channelID] = make(map[*Client]bool)
			}
			h.channels[req.channelID][req.client] = true
			req.client.subsMu.Lock()
			req.client.subs[req.channelID] = true
			req.client.subsMu.Unlock()

		case req := <-h.unsub:
			if members, ok := h.channels[req.channelID]; ok {
				delete(members, req.client)
				if len(members) == 0 {
					delete(h.channels, req.channelID)
				}
			}
			req.client.subsMu.Lock()
			delete(req.client.subs, req.channelID)
			req.client.subsMu.Unlock()

		case msg := <-h.broadcast:
			if members, ok := h.channels[msg.channelID]; ok {
				var overflow []*Client
				for client := range members {
					select {
					case client.send <- msg.data:
					default:
						overflow = append(overflow, client)
					}
				}
				for _, client := range overflow {
					userID := client.UserID
					username := client.Username
					wasLast := h.removeClient(client)
					if wasLast {
						update, _ := json.Marshal(map[string]any{
							"type":     "presence_update",
							"user_id":  userID,
							"username": username,
							"status":   "offline",
						})
						h.broadcastAll(update)
					}
				}
			}

		case msg := <-h.directSend:
			select {
			case msg.client.send <- msg.data:
			default:
			}

		case msg := <-h.broadcastExc:
			if members, ok := h.channels[msg.channelID]; ok {
				var overflow []*Client
				for client := range members {
					if client == msg.exclude {
						continue
					}
					select {
					case client.send <- msg.data:
					default:
						overflow = append(overflow, client)
					}
				}
				for _, client := range overflow {
					userID := client.UserID
					username := client.Username
					wasLast := h.removeClient(client)
					if wasLast {
						update, _ := json.Marshal(map[string]any{
							"type":     "presence_update",
							"user_id":  userID,
							"username": username,
							"status":   "offline",
						})
						h.broadcastAll(update)
					}
				}
			}

		case data := <-h.globalBroadcast:
			h.broadcastAll(data)

		case msg := <-h.userSend:
			if conns, ok := h.onlineUsers[msg.userID]; ok {
				var overflow []*Client
				for client := range conns {
					select {
					case client.send <- msg.data:
					default:
						overflow = append(overflow, client)
					}
				}
				for _, client := range overflow {
					userID := client.UserID
					username := client.Username
					wasLast := h.removeClient(client)
					if wasLast {
						update, _ := json.Marshal(map[string]any{
							"type":     "presence_update",
							"user_id":  userID,
							"username": username,
							"status":   "offline",
						})
						h.broadcastAll(update)
					}
				}
			}

		case action := <-h.voiceJoin:
			userID := action.client.UserID
			// Remove from any existing voice channel first (both state and WebRTC)
			for channelID, users := range h.voiceUsers {
				if _, ok := users[userID]; ok {
					if h.VoiceManager != nil {
						h.VoiceManager.LeaveRoom(channelID, userID)
					}
					break
				}
			}
			h.removeUserFromVoiceNoNotify(userID)
			// Add to new channel
			if _, ok := h.voiceUsers[action.channelID]; !ok {
				h.voiceUsers[action.channelID] = make(map[uuid.UUID]*VoiceUserState)
			}
			h.voiceUsers[action.channelID][userID] = &VoiceUserState{}

			// Create WebRTC PeerConnection via the voice manager
			if h.VoiceManager != nil {
				sendFunc := func(data []byte) {
					h.SendToUser(userID, data)
				}
				if _, err := h.VoiceManager.JoinRoom(action.channelID, userID, action.client.Username, sendFunc); err != nil {
					log.Printf("voice: failed to join room for user %s: %v", userID, err)
				}
			}

			h.broadcastVoiceState()

		case action := <-h.voiceLeave:
			userID := action.client.UserID
			h.removeUserFromVoice(userID)
			_ = action // channelID not needed, we remove from whichever channel they're in

		case change := <-h.voiceState:
			// Find the user in any voice channel and update their state
			for _, users := range h.voiceUsers {
				if state, ok := users[change.userID]; ok {
					if change.muted != nil {
						state.Muted = *change.muted
					}
					if change.deafened != nil {
						state.Deafened = *change.deafened
					}
					if change.streaming != nil {
						state.Streaming = *change.streaming
					}
					h.broadcastVoiceState()
					break
				}
			}

		case msg := <-h.voiceBroadcast:
			// Broadcast to all users in a specific voice channel
			if users, ok := h.voiceUsers[msg.channelID]; ok {
				for userID := range users {
					if conns, connOk := h.onlineUsers[userID]; connOk {
						for client := range conns {
							select {
							case client.send <- msg.data:
							default:
							}
						}
					}
				}
			}

		case userID := <-h.kickUser:
			if conns, ok := h.onlineUsers[userID]; ok {
				// Collect clients to remove (can't modify map while iterating)
				clients := make([]*Client, 0, len(conns))
				for client := range conns {
					clients = append(clients, client)
				}
				for _, client := range clients {
					h.removeClient(client)
					client.conn.Close()
				}
				// Broadcast offline status
				update, _ := json.Marshal(map[string]any{
					"type":    "presence_update",
					"user_id": userID,
					"status":  "offline",
				})
				h.broadcastAll(update)
			}
		}
	}
}

func (h *Hub) Subscribe(client *Client, channelID uuid.UUID) {
	h.subscribe <- subRequest{client: client, channelID: channelID}
}

func (h *Hub) Unsubscribe(client *Client, channelID uuid.UUID) {
	h.unsub <- subRequest{client: client, channelID: channelID}
}

func (h *Hub) Broadcast(channelID uuid.UUID, data []byte) {
	h.broadcast <- broadcastMsg{channelID: channelID, data: data}
}

func (h *Hub) BroadcastExclude(channelID uuid.UUID, data []byte, exclude *Client) {
	h.broadcastExc <- broadcastExcludeMsg{channelID: channelID, data: data, exclude: exclude}
}

func (h *Hub) BroadcastGlobal(data []byte) {
	h.globalBroadcast <- data
}

func (h *Hub) SendToUser(userID uuid.UUID, data []byte) {
	h.userSend <- userMsg{userID: userID, data: data}
}

func (h *Hub) VoiceJoin(client *Client, channelID uuid.UUID) {
	h.voiceJoin <- voiceAction{client: client, channelID: channelID}
}

func (h *Hub) VoiceLeave(client *Client) {
	h.voiceLeave <- voiceAction{client: client}
}

func (h *Hub) KickUser(userID uuid.UUID) {
	h.kickUser <- userID
}

func (h *Hub) UpdateVoiceState(userID uuid.UUID, muted, deafened, streaming *bool) {
	h.voiceState <- voiceStateChange{
		userID:    userID,
		muted:     muted,
		deafened:  deafened,
		streaming: streaming,
	}
}

func (h *Hub) BroadcastToVoiceChannel(channelID uuid.UUID, data []byte) {
	h.voiceBroadcast <- voiceBroadcastMsg{channelID: channelID, data: data}
}
