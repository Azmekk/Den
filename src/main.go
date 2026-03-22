package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/pion/webrtc/v4"

	"github.com/Azmekk/den/internal/db"
	"github.com/Azmekk/den/internal/router"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/voice"
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
		log.Fatal("JWT_SECRET is required (32+ characters recommended)")
	}

	// Optional SMTP configuration
	var smtpConfig *service.SMTPConfig
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost != "" {
		smtpPort := 587
		if portStr := os.Getenv("SMTP_PORT"); portStr != "" {
			if parsed, parseError := strconv.Atoi(portStr); parseError == nil {
				smtpPort = parsed
			}
		}
		smtpConfig = &service.SMTPConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     os.Getenv("SMTP_FROM"),
		}
		log.Printf("SMTP configured (%s:%d)", smtpHost, smtpPort)
	} else {
		log.Println("SMTP not configured, email features disabled")
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	conn, openError := sql.Open("postgres", dbURL)
	if openError != nil {
		log.Fatalf("failed to connect to database: %v", openError)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to database")

	if err := runMigrations(conn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	queries := db.New(conn)

	bucketSvc := service.NewBucketService()
	if bucketSvc != nil {
		log.Println("bucket storage configured")
	} else {
		log.Println("bucket storage not configured, uploads disabled")
	}

	hub := ws.NewHub()
	go hub.Run()

	authSvc := service.NewAuthService(queries, jwtSecret, smtpConfig, frontendURL)
	channelSvc := service.NewChannelService(queries)
	emoteSvc := service.NewEmoteService(queries, bucketSvc)
	adminSvc := service.NewAdminService(queries, authSvc, hub)
	if err := adminSvc.LoadSettings(context.Background()); err != nil {
		log.Fatalf("failed to load admin settings: %v", err)
	}
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

	// Voice (Pion WebRTC SFU)
	var voiceManager *voice.Manager
	{
		stunServers := os.Getenv("STUN_SERVERS")
		if stunServers == "" {
			stunServers = "stun:stun.l.google.com:19302"
		}

		var iceServers []webrtc.ICEServer
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs: strings.Split(stunServers, ","),
		})

		turnURL := os.Getenv("TURN_URL")
		turnUsername := os.Getenv("TURN_USERNAME")
		turnCredential := os.Getenv("TURN_CREDENTIAL")
		if turnURL != "" {
			iceServers = append(iceServers, webrtc.ICEServer{
				URLs:       []string{turnURL},
				Username:   turnUsername,
				Credential: turnCredential,
			})
		}

		voiceManager = voice.NewManager(iceServers)
		hub.VoiceManager = voiceManager
		log.Println("voice channels enabled (Pion WebRTC SFU)")
	}

	{
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go adminSvc.RunMessageCleanupLoop(ctx, 5000, 1*time.Hour)
		log.Println("message cleanup loop started (hourly check, limit from DB)")
	}

	allowedOrigins := parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"))
	if len(allowedOrigins) > 0 {
		log.Printf("WebSocket allowed origins: %v", allowedOrigins)
	} else {
		log.Println("ALLOWED_ORIGINS not set, WebSocket will use same-origin check")
	}

	staticFS, fsError := fs.Sub(StaticFiles, "web/build")
	if fsError != nil {
		log.Fatalf("failed to create sub filesystem: %v", fsError)
	}

	appRouter := router.New(authSvc, channelSvc, messageSvc, userSvc, adminSvc, emoteSvc, dmSvc, mediaSvc, voiceManager, hub, staticFS, bucketSvc != nil, allowedOrigins)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, appRouter); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func runMigrations(conn *sql.DB) error {
	migrationFS, subError := fs.Sub(MigrationFiles, "db/migrations")
	if subError != nil {
		return fmt.Errorf("creating migration sub-fs: %w", subError)
	}

	source, sourceError := iofs.New(migrationFS, ".")
	if sourceError != nil {
		return fmt.Errorf("creating migration source: %w", sourceError)
	}

	driver, driverError := postgres.WithInstance(conn, &postgres.Config{})
	if driverError != nil {
		return fmt.Errorf("creating migration driver: %w", driverError)
	}

	migrator, migratorError := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if migratorError != nil {
		return fmt.Errorf("creating migrate instance: %w", migratorError)
	}

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
	}

	version, dirty, _ := migrator.Version()
	log.Printf("migrations complete (version=%d, dirty=%v)", version, dirty)
	return nil
}
