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
	"time"

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
	adminSvc := service.NewAdminService(queries, authSvc)
	if err := adminSvc.LoadSettings(context.Background()); err != nil {
		log.Fatalf("failed to load admin settings: %v", err)
	}
	authSvc.SetInviteValidator(adminSvc.ValidateAndUseInviteCode)
	messageSvc := service.NewMessageService(queries, emoteSvc, adminSvc.GetMaxMessageChars)
	dmSvc := service.NewDMService(queries, emoteSvc, adminSvc.GetMaxMessageChars)
	userSvc := service.NewUserService(queries)

	var mediaSvc *service.MediaService
	if bucketSvc != nil {
		mediaSvc = service.NewMediaService(queries, bucketSvc)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go mediaSvc.RunCleanupLoop(ctx)
		log.Println("media upload enabled, cleanup loop started")
	}

	voiceSvc := service.NewVoiceService(
		os.Getenv("LIVEKIT_API_KEY"),
		os.Getenv("LIVEKIT_API_SECRET"),
		os.Getenv("LIVEKIT_PUBLIC_URL"),
	)
	if voiceSvc != nil {
		log.Println("voice channels enabled (LiveKit configured)")
	} else {
		log.Println("voice channels disabled (LIVEKIT_* env vars not set)")
	}

	{
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go adminSvc.RunMessageCleanupLoop(ctx, 5000, 1*time.Hour)
		log.Println("message cleanup loop started (hourly check, limit from DB)")
	}

	unfurlSvc := service.NewUnfurlService(os.Getenv("UNFURL_USER_AGENT"))

	hub := ws.NewHub()
	go hub.Run()

	staticFS, err := fs.Sub(StaticFiles, "web/build")
	if err != nil {
		log.Fatalf("failed to create sub filesystem: %v", err)
	}

	r := router.New(authSvc, channelSvc, messageSvc, userSvc, adminSvc, emoteSvc, dmSvc, mediaSvc, voiceSvc, unfurlSvc, hub, staticFS, bucketSvc != nil)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
