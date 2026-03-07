package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/martinmckenna/den/src"
	"github.com/martinmckenna/den/src/internal/auth"
	"github.com/martinmckenna/den/src/internal/channel"
	"github.com/martinmckenna/den/src/internal/db"
	"github.com/martinmckenna/den/src/internal/message"
	"github.com/martinmckenna/den/src/internal/ws"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://den:changeme@localhost:5432/den?sslmode=disable"
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret-change-me"
		log.Println("WARNING: using default JWT_SECRET, set JWT_SECRET env var in production")
	}

	openRegistration := strings.ToLower(os.Getenv("OPEN_REGISTRATION")) != "false"

	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to database")

	queries := db.New(conn)
	authService := auth.NewService(queries, jwtSecret, openRegistration)
	channelService := channel.NewService(queries)
	messageService := message.NewService(queries)
	hub := ws.NewHub()
	go hub.Run()

	mux := http.NewServeMux()

	// Auth routes (public)
	mux.HandleFunc("POST /api/register", authService.RegisterHandler)
	mux.HandleFunc("POST /api/login", authService.LoginHandler)
	mux.HandleFunc("POST /api/refresh", authService.RefreshHandler)
	mux.HandleFunc("POST /api/logout", authService.LogoutHandler)

	// Protected routes
	mux.Handle("GET /api/me", authService.RequireAuth(http.HandlerFunc(authService.MeHandler)))
	mux.Handle("POST /api/change-password", authService.RequireAuth(http.HandlerFunc(authService.ChangePasswordHandler)))

	// Channel CRUD
	mux.Handle("GET /api/channels", authService.RequireAuth(http.HandlerFunc(channelService.ListHandler)))
	mux.Handle("GET /api/channels/{id}", authService.RequireAuth(http.HandlerFunc(channelService.GetHandler)))
	mux.Handle("POST /api/channels", authService.RequireAdmin(http.HandlerFunc(channelService.CreateHandler)))
	mux.Handle("PUT /api/channels/{id}", authService.RequireAdmin(http.HandlerFunc(channelService.UpdateHandler)))
	mux.Handle("DELETE /api/channels/{id}", authService.RequireAdmin(http.HandlerFunc(channelService.DeleteHandler)))

	// Message history
	mux.Handle("GET /api/channels/{id}/messages", authService.RequireAuth(http.HandlerFunc(messageService.GetHistoryHandler)))

	// WebSocket (auth via query param, not middleware)
	mux.HandleFunc("GET /api/ws", ws.ServeWS(hub, authService, messageService))

	// Static files
	staticFS, err := fs.Sub(src.StaticFiles, "web/build")
	if err != nil {
		log.Fatalf("failed to create sub filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
