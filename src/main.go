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
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
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

	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		log.Fatal("SUPABASE_URL is required")
	}

	supabaseServiceRoleKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if supabaseServiceRoleKey == "" {
		log.Fatal("SUPABASE_SERVICE_ROLE_KEY is required")
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

	authSvc := service.NewAuthService(queries, supabaseURL, supabaseServiceRoleKey)
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

	voiceSvc := service.NewVoiceService(
		os.Getenv("LIVEKIT_API_KEY"),
		os.Getenv("LIVEKIT_API_SECRET"),
		os.Getenv("LIVEKIT_PUBLIC_URL"),
		os.Getenv("LIVEKIT_URL"),
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

	staticFS, fsError := fs.Sub(StaticFiles, "web/build")
	if fsError != nil {
		log.Fatalf("failed to create sub filesystem: %v", fsError)
	}

	appRouter := router.New(authSvc, channelSvc, messageSvc, userSvc, adminSvc, emoteSvc, dmSvc, mediaSvc, voiceSvc, unfurlSvc, hub, staticFS, bucketSvc != nil)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, appRouter); err != nil {
		log.Fatalf("server error: %v", err)
	}
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
