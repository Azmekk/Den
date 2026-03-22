package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Azmekk/den/internal/voice"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	UserID      uuid.UUID
	Username    string
	DisplayName string
	IsAdmin     bool
	IsNewUser   bool
	msgHandler    MessageHandler
	dmHandler     DMMessageHandler
	voiceManager  *voice.Manager
	subsMu        sync.RWMutex
	subs          map[uuid.UUID]bool
}

func newClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, username string, displayName string, isAdmin bool, isNewUser bool, msgHandler MessageHandler, dmHandler DMMessageHandler, voiceManager *voice.Manager) *Client {
	return &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, 256),
		UserID:       userID,
		Username:     username,
		DisplayName:  displayName,
		IsAdmin:      isAdmin,
		IsNewUser:    isNewUser,
		msgHandler:   msgHandler,
		dmHandler:    dmHandler,
		voiceManager: voiceManager,
		subs:         make(map[uuid.UUID]bool),
	}
}

// IsSubscribed checks whether the client is subscribed to the given channel.
// Safe for concurrent use from the client's read goroutine while the hub goroutine writes subs.
func (client *Client) IsSubscribed(channelID uuid.UUID) bool {
	client.subsMu.RLock()
	defer client.subsMu.RUnlock()
	return client.subs[channelID]
}

type incomingMessage struct {
	Type      string          `json:"type"`
	ChannelID uuid.UUID       `json:"channel_id"`
	DMPairID  uuid.UUID       `json:"dm_pair_id"`
	MessageID uuid.UUID       `json:"message_id"`
	Content   string          `json:"content"`
	ReplyToID string          `json:"reply_to_id"`
	SDP       json.RawMessage `json:"sdp,omitempty"`
	Candidate json.RawMessage `json:"candidate,omitempty"`
	Muted     *bool           `json:"muted,omitempty"`
	Speaking  *bool           `json:"speaking,omitempty"`
	Deafened  *bool           `json:"deafened,omitempty"`
	Streaming *bool           `json:"streaming,omitempty"`
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
		if !c.IsSubscribed(msg.ChannelID) {
			c.sendError("not subscribed to channel")
			return
		}
		var replyToID *uuid.UUID
		if msg.ReplyToID != "" {
			parsed, parseErr := uuid.Parse(msg.ReplyToID)
			if parseErr == nil {
				replyToID = &parsed
			}
		}
		data, _, err := c.msgHandler.SendMessage(ctx, msg.ChannelID, c.UserID, c.Username, msg.Content, replyToID)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		c.hub.BroadcastGlobal(data)

		// Auto-stop typing indicator when a message is sent
		stopEnvelope, _ := json.Marshal(map[string]any{
			"type":       "typing_stop",
			"channel_id": msg.ChannelID,
			"user_id":    c.UserID,
			"username":   c.Username,
		})
		c.hub.BroadcastExclude(msg.ChannelID, stopEnvelope, c)

	case "send_dm":
		otherUserID, err := c.dmHandler.ValidateUserInPair(ctx, msg.DMPairID, c.UserID)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		var replyToID *uuid.UUID
		if msg.ReplyToID != "" {
			parsed, parseErr := uuid.Parse(msg.ReplyToID)
			if parseErr == nil {
				replyToID = &parsed
			}
		}
		data, _, err := c.dmHandler.SendDMMessage(ctx, msg.DMPairID, c.UserID, c.Username, msg.Content, replyToID)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		c.hub.SendToUser(c.UserID, data)
		c.hub.SendToUser(otherUserID, data)

	case "edit_message":
		data, channelID, dmPairID, err := c.msgHandler.EditMessage(ctx, msg.MessageID, c.UserID, msg.Content)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		if dmPairID != uuid.Nil {
			// DM message: send to both users
			otherUserID, validateError := c.dmHandler.ValidateUserInPair(ctx, dmPairID, c.UserID)
			if validateError != nil {
				log.Printf("ws: edit_message DM broadcast failed: ValidateUserInPair(%s, %s) returned error after mutation: %v", dmPairID, c.UserID, validateError)
			} else {
				c.hub.SendToUser(c.UserID, data)
				c.hub.SendToUser(otherUserID, data)
			}
		} else {
			_ = channelID
			c.hub.BroadcastGlobal(data)
		}

	case "delete_message":
		channelID, dmPairID, err := c.msgHandler.DeleteMessage(ctx, msg.MessageID, c.UserID, c.IsAdmin)
		if err != nil {
			c.sendError(err.Error())
			return
		}
		deleteEnvelope := map[string]any{
			"type": "delete_message",
			"id":   msg.MessageID,
		}
		if dmPairID != uuid.Nil {
			deleteEnvelope["dm_pair_id"] = dmPairID
			data, _ := json.Marshal(deleteEnvelope)
			otherUserID, validateError := c.dmHandler.ValidateUserInPair(ctx, dmPairID, c.UserID)
			if validateError != nil {
				log.Printf("ws: delete_message DM broadcast failed: ValidateUserInPair(%s, %s) returned error after mutation: %v", dmPairID, c.UserID, validateError)
			} else {
				c.hub.SendToUser(c.UserID, data)
				c.hub.SendToUser(otherUserID, data)
			}
		} else {
			deleteEnvelope["channel_id"] = channelID
			data, _ := json.Marshal(deleteEnvelope)
			c.hub.BroadcastGlobal(data)
		}

	case "voice_join":
		c.hub.VoiceJoin(c, msg.ChannelID)

	case "voice_leave":
		c.hub.VoiceLeave(c)

	case "voice_offer":
		if c.voiceManager == nil {
			c.sendError("voice not enabled")
			return
		}
		answerData, err := c.voiceManager.HandleOffer(msg.ChannelID, c.UserID, msg.SDP)
		if err != nil {
			log.Printf("ws: voice_offer error for user %s: %v", c.UserID, err)
			c.sendError("voice offer failed")
			return
		}
		select {
		case c.send <- answerData:
		default:
		}

	case "voice_answer":
		if c.voiceManager == nil {
			return
		}
		if err := c.voiceManager.HandleAnswer(msg.ChannelID, c.UserID, msg.SDP); err != nil {
			log.Printf("ws: voice_answer error for user %s: %v", c.UserID, err)
		}

	case "voice_ice_candidate":
		if c.voiceManager == nil {
			return
		}
		if err := c.voiceManager.HandleICECandidate(msg.ChannelID, c.UserID, msg.Candidate); err != nil {
			log.Printf("ws: voice_ice_candidate error for user %s: %v", c.UserID, err)
		}

	case "voice_mute_state":
		if msg.Muted == nil {
			return
		}
		c.hub.UpdateVoiceState(c.UserID, msg.Muted, nil, nil)

	case "voice_deafen_state":
		if msg.Deafened == nil {
			return
		}
		if *msg.Deafened {
			// Deafening auto-mutes
			muted := true
			c.hub.UpdateVoiceState(c.UserID, &muted, msg.Deafened, nil)
		} else {
			c.hub.UpdateVoiceState(c.UserID, nil, msg.Deafened, nil)
		}

	case "voice_streaming_state":
		if msg.Streaming == nil {
			return
		}
		c.hub.UpdateVoiceState(c.UserID, nil, nil, msg.Streaming)

	case "voice_speaking":
		if msg.Speaking == nil {
			return
		}
		envelope, _ := json.Marshal(map[string]any{
			"type":       "voice_speaking",
			"channel_id": msg.ChannelID,
			"user_id":    c.UserID,
			"speaking":   *msg.Speaking,
		})
		c.hub.BroadcastToVoiceChannel(msg.ChannelID, envelope)

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
