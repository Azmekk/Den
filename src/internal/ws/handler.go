package ws

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Azmekk/den/internal/service"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(request *http.Request) bool {
		return true
	},
}

type authMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

func writeError(conn *websocket.Conn, errorMsg string) {
	msg, _ := json.Marshal(map[string]string{"type": "auth_error", "error": errorMsg})
	conn.WriteMessage(websocket.TextMessage, msg)
	conn.Close()
}

func ServeWS(hub *Hub, authService *service.AuthService, msgHandler MessageHandler, dmHandler DMMessageHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		conn, upgradeError := upgrader.Upgrade(writer, request, nil)
		if upgradeError != nil {
			log.Printf("ws upgrade error: %v", upgradeError)
			return
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		_, raw, readError := conn.ReadMessage()
		if readError != nil {
			writeError(conn, "expected auth message")
			return
		}

		var auth authMessage
		if err := json.Unmarshal(raw, &auth); err != nil || auth.Type != "auth" || auth.Token == "" {
			writeError(conn, "invalid auth message")
			return
		}

		claims, validationError := authService.ValidateSupabaseToken(auth.Token)
		if validationError != nil {
			if errors.Is(validationError, service.ErrUserBanned) {
				writeError(conn, "account is banned")
			} else {
				writeError(conn, "invalid or expired token")
			}
			return
		}

		// Look up or create Den user from Supabase claims
		user, syncError := authService.SyncUser(request.Context(), claims)
		if syncError != nil {
			if errors.Is(syncError, service.ErrUserBanned) {
				writeError(conn, "account is banned")
			} else {
				writeError(conn, "failed to resolve user")
			}
			return
		}

		okMsg, _ := json.Marshal(map[string]string{"type": "auth_ok"})
		if err := conn.WriteMessage(websocket.TextMessage, okMsg); err != nil {
			conn.Close()
			return
		}

		conn.SetReadDeadline(time.Time{})

		client := newClient(hub, conn, user.ID, user.Username, user.IsAdmin, msgHandler, dmHandler)
		hub.register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}
