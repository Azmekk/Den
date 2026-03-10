package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/Azmekk/den/internal/db"
	"github.com/Azmekk/den/internal/router"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/ws"
)

func main() {
	_ = godotenv.Load("../.env", ".env")

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

	bucketSvc := service.NewBucketService()
	if bucketSvc != nil {
		log.Println("bucket storage configured")
	} else {
		log.Println("bucket storage not configured, uploads disabled")
	}

	authSvc := service.NewAuthService(queries, jwtSecret, openRegistration)
	channelSvc := service.NewChannelService(queries)
	emoteSvc := service.NewEmoteService(queries, bucketSvc)
	messageSvc := service.NewMessageService(queries, emoteSvc)
	dmSvc := service.NewDMService(queries, emoteSvc)
	userSvc := service.NewUserService(queries)
	adminSvc := service.NewAdminService(queries, authSvc)

	var mediaSvc *service.MediaService
	if bucketSvc != nil {
		mediaSvc = service.NewMediaService(queries, bucketSvc)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go mediaSvc.RunCleanupLoop(ctx)
		log.Println("media upload enabled, cleanup loop started")
	}

	hub := ws.NewHub()
	go hub.Run()

	staticFS, err := fs.Sub(StaticFiles, "web/build")
	if err != nil {
		log.Fatalf("failed to create sub filesystem: %v", err)
	}

	r := router.New(authSvc, channelSvc, messageSvc, userSvc, adminSvc, emoteSvc, dmSvc, mediaSvc, hub, staticFS, bucketSvc != nil)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
