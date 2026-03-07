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
	"github.com/martinmckenna/den/src/internal/db"

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

	mux := http.NewServeMux()

	// Auth routes (public)
	mux.HandleFunc("POST /api/register", authService.RegisterHandler)
	mux.HandleFunc("POST /api/login", authService.LoginHandler)
	mux.HandleFunc("POST /api/refresh", authService.RefreshHandler)
	mux.HandleFunc("POST /api/logout", authService.LogoutHandler)

	// Protected routes
	mux.Handle("GET /api/me", authService.RequireAuth(http.HandlerFunc(authService.MeHandler)))
	mux.Handle("POST /api/change-password", authService.RequireAuth(http.HandlerFunc(authService.ChangePasswordHandler)))

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
