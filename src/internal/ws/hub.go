package ws

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type MessageHandler interface {
	SendMessage(ctx context.Context, channelID, userID uuid.UUID, username, content string) ([]byte, []uuid.UUID, error)
	EditMessage(ctx context.Context, messageID, userID uuid.UUID, content string) ([]byte, uuid.UUID, error)
	DeleteMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) (uuid.UUID, error)
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

type Hub struct {
	clients         map[*Client]bool
	channels        map[uuid.UUID]map[*Client]bool
	onlineUsers     map[uuid.UUID]map[*Client]bool
	register        chan *Client
	unregister      chan *Client
	subscribe       chan subRequest
	unsub           chan subRequest
	broadcast       chan broadcastMsg
	directSend      chan directMsg
	broadcastExc    chan broadcastExcludeMsg
	globalBroadcast chan []byte
	userSend        chan userMsg
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
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		subscribe:       make(chan subRequest),
		unsub:           make(chan subRequest),
		broadcast:       make(chan broadcastMsg, 256),
		directSend:      make(chan directMsg, 256),
		broadcastExc:    make(chan broadcastExcludeMsg, 256),
		globalBroadcast: make(chan []byte, 256),
		userSend:        make(chan userMsg, 256),
	}
}

func (h *Hub) broadcastAll(data []byte) {
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			h.removeClient(client)
		}
	}
}

func (h *Hub) removeClient(client *Client) {
	delete(h.clients, client)
	close(client.send)
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
		}
	}
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
				h.removeClient(client)

				// Broadcast offline status if no connections remain
				if len(h.onlineUsers[userID]) == 0 {
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
			req.client.subs[req.channelID] = true

		case req := <-h.unsub:
			if members, ok := h.channels[req.channelID]; ok {
				delete(members, req.client)
				if len(members) == 0 {
					delete(h.channels, req.channelID)
				}
			}
			delete(req.client.subs, req.channelID)

		case msg := <-h.broadcast:
			if members, ok := h.channels[msg.channelID]; ok {
				for client := range members {
					select {
					case client.send <- msg.data:
					default:
						h.removeClient(client)
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
				for client := range members {
					if client == msg.exclude {
						continue
					}
					select {
					case client.send <- msg.data:
					default:
						h.removeClient(client)
					}
				}
			}

		case data := <-h.globalBroadcast:
			h.broadcastAll(data)

		case msg := <-h.userSend:
			if conns, ok := h.onlineUsers[msg.userID]; ok {
				for client := range conns {
					select {
					case client.send <- msg.data:
					default:
						h.removeClient(client)
					}
				}
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
