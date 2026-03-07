package ws

import (
	"context"

	"github.com/google/uuid"
)

type MessageHandler interface {
	SendMessage(ctx context.Context, channelID, userID uuid.UUID, username, content string) ([]byte, error)
	EditMessage(ctx context.Context, messageID, userID uuid.UUID, content string) ([]byte, uuid.UUID, error)
	DeleteMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) (uuid.UUID, error)
}

type subRequest struct {
	client    *Client
	channelID uuid.UUID
}

type Hub struct {
	clients    map[*Client]bool
	channels   map[uuid.UUID]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	subscribe  chan subRequest
	unsub      chan subRequest
	broadcast  chan broadcastMsg
}

type broadcastMsg struct {
	channelID uuid.UUID
	data      []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		channels:   make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		subscribe:  make(chan subRequest),
		unsub:      make(chan subRequest),
		broadcast:  make(chan broadcastMsg, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				for chID := range client.subs {
					if members, ok := h.channels[chID]; ok {
						delete(members, client)
						if len(members) == 0 {
							delete(h.channels, chID)
						}
					}
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
