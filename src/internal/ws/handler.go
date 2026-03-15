package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Azmekk/den/internal/service"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type authMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

func writeError(conn *websocket.Conn, errMsg string) {
	msg, _ := json.Marshal(map[string]string{"type": "auth_error", "error": errMsg})
	conn.WriteMessage(websocket.TextMessage, msg)
	conn.Close()
}

func ServeWS(hub *Hub, authService *service.AuthService, msgHandler MessageHandler, dmHandler DMMessageHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade error: %v", err)
			return
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		_, raw, err := conn.ReadMessage()
		if err != nil {
			writeError(conn, "expected auth message")
			return
		}

		var auth authMessage
		if err := json.Unmarshal(raw, &auth); err != nil || auth.Type != "auth" || auth.Token == "" {
			writeError(conn, "invalid auth message")
			return
		}

		claims, err := authService.ValidateAccessToken(auth.Token)
		if err != nil {
			writeError(conn, "invalid or expired token")
			return
		}

		sub, _ := claims["sub"].(string)
		userID, err := uuid.Parse(sub)
		if err != nil {
			writeError(conn, "invalid token claims")
			return
		}
		username, _ := claims["username"].(string)
		isAdmin, _ := claims["is_admin"].(bool)

		okMsg, _ := json.Marshal(map[string]string{"type": "auth_ok"})
		if err := conn.WriteMessage(websocket.TextMessage, okMsg); err != nil {
			conn.Close()
			return
		}

		conn.SetReadDeadline(time.Time{})

		client := newClient(hub, conn, userID, username, isAdmin, msgHandler, dmHandler)
		hub.register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}
