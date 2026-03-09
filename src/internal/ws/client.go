package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	UserID     uuid.UUID
	Username   string
	IsAdmin    bool
	msgHandler MessageHandler
	subs       map[uuid.UUID]bool
}

func newClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, username string, isAdmin bool, msgHandler MessageHandler) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, 256),
		UserID:     userID,
		Username:   username,
		IsAdmin:    isAdmin,
		msgHandler: msgHandler,
		subs:       make(map[uuid.UUID]bool),
	}
}

type incomingMessage struct {
	Type      string    `json:"type"`
	ChannelID uuid.UUID `json:"channel_id"`
	MessageID uuid.UUID `json:"message_id"`
	Content   string    `json:"content"`
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg incomingMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.sendError("invalid JSON")
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg incomingMessage) {
	ctx := context.Background()

	switch msg.Type {
	case "subscribe":
		c.hub.Subscribe(c, msg.ChannelID)

	case "unsubscribe":
		c.hub.Unsubscribe(c, msg.ChannelID)

	case "send_message":
		data, err := c.msgHandler.SendMessage(ctx, msg.ChannelID, c.UserID, c.Username, msg.Content)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		c.hub.Broadcast(msg.ChannelID, data)

		// Auto-stop typing indicator when a message is sent
		stopEnvelope, _ := json.Marshal(map[string]any{
			"type":       "typing_stop",
			"channel_id": msg.ChannelID,
			"user_id":    c.UserID,
			"username":   c.Username,
		})
		c.hub.BroadcastExclude(msg.ChannelID, stopEnvelope, c)

	case "edit_message":
		data, channelID, err := c.msgHandler.EditMessage(ctx, msg.MessageID, c.UserID, msg.Content)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		c.hub.Broadcast(channelID, data)

	case "delete_message":
		channelID, err := c.msgHandler.DeleteMessage(ctx, msg.MessageID, c.UserID, c.IsAdmin)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		envelope, _ := json.Marshal(map[string]any{
			"type":       "delete_message",
			"id":         msg.MessageID,
			"channel_id": channelID,
		})
		c.hub.Broadcast(channelID, envelope)

	case "typing_start":
		envelope, _ := json.Marshal(map[string]any{
			"type":       "typing_start",
			"channel_id": msg.ChannelID,
			"user_id":    c.UserID,
			"username":   c.Username,
		})
		c.hub.BroadcastExclude(msg.ChannelID, envelope, c)

	case "typing_stop":
		envelope, _ := json.Marshal(map[string]any{
			"type":       "typing_stop",
			"channel_id": msg.ChannelID,
			"user_id":    c.UserID,
			"username":   c.Username,
		})
		c.hub.BroadcastExclude(msg.ChannelID, envelope, c)

	default:
		c.sendError("unknown message type: " + msg.Type)
	}
}

func (c *Client) sendError(msg string) {
	envelope, err := json.Marshal(map[string]string{
		"type":  "error",
		"error": msg,
	})
	if err != nil {
		log.Printf("ws: failed to marshal error: %v", err)
		return
	}
	select {
	case c.send <- envelope:
	default:
	}
}
