package ws

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Azmekk/den/internal/service"
)

func newUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     makeCheckOrigin(allowedOrigins),
	}
}

func makeCheckOrigin(allowedOrigins []string) func(*http.Request) bool {
	if len(allowedOrigins) > 0 {
		originSet := make(map[string]bool, len(allowedOrigins))
		for _, origin := range allowedOrigins {
			originSet[strings.ToLower(origin)] = true
		}
		return func(request *http.Request) bool {
			origin := request.Header.Get("Origin")
			if origin == "" {
				return true // non-browser clients (e.g. CLI tools) don't send Origin
			}
			return originSet[strings.ToLower(origin)]
		}
	}

	// Default: same-origin check
	return func(request *http.Request) bool {
		origin := request.Header.Get("Origin")
		if origin == "" {
			return true
		}
		parsedOrigin, parseError := url.Parse(origin)
		if parseError != nil {
			return false
		}
		return strings.EqualFold(parsedOrigin.Host, request.Host)
	}
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

func ServeWS(hub *Hub, authService *service.AuthService, msgHandler MessageHandler, dmHandler DMMessageHandler, allowedOrigins []string) http.HandlerFunc {
	upgrader := newUpgrader(allowedOrigins)
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
		user, isNewUser, syncError := authService.SyncUser(request.Context(), claims)
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

		displayName := ""
		if user.DisplayName.Valid {
			displayName = user.DisplayName.String
		}

		client := newClient(hub, conn, user.ID, user.Username, displayName, user.IsAdmin, isNewUser, msgHandler, dmHandler)
		hub.register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}
