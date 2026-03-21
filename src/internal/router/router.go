package router

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/Azmekk/den/internal/handler"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/voice"
	"github.com/Azmekk/den/internal/ws"
)

func New(authSvc *service.AuthService, channelSvc *service.ChannelService, messageSvc *service.MessageService, userSvc *service.UserService, adminSvc *service.AdminService, emoteSvc *service.EmoteService, dmSvc *service.DMService, mediaSvc *service.MediaService, voiceManager *voice.Manager, hub *ws.Hub, staticFS fs.FS, bucketConfigured bool, supabaseURL string, supabaseAnonKey string, allowedOrigins []string) chi.Router {
	authHandler := handler.NewAuthHandler(authSvc)
	channelHandler := handler.NewChannelHandler(channelSvc)
	messageHandler := handler.NewMessageHandler(messageSvc, hub)
	userHandler := handler.NewUserHandler(userSvc, mediaSvc, hub)
	adminHandler := handler.NewAdminHandler(adminSvc, mediaSvc)
	emoteHandler := handler.NewEmoteHandler(emoteSvc, hub)
	configHandler := handler.NewConfigHandler(bucketConfigured, voiceManager != nil, voiceManager, adminSvc.GetMaxMessageChars, authSvc.IsOpenRegistration, supabaseURL, supabaseAnonKey)
	dmHandler := handler.NewDMHandler(dmSvc)
	var mediaHandler *handler.MediaHandler
	if mediaSvc != nil {
		mediaHandler = handler.NewMediaHandler(mediaSvc)
	}
	exportHandler := handler.NewExportHandler(channelSvc, userSvc, emoteSvc, dmSvc, userSvc.Queries(), authSvc.GetInstanceName)

	router := chi.NewRouter()

	router.Use(cloudflareRealIP)
	router.Use(chimw.RealIP)
	router.Use(chimw.RequestID)
	router.Use(chimw.Logger)
	router.Use(chimw.Recoverer)
	router.Use(chimw.Compress(5))
	router.Use(chimw.Heartbeat("/healthz"))

	router.Route("/api", func(router chi.Router) {
		// Public
		router.Get("/config", configHandler.GetConfig)
		router.Get("/emotes/{id}/image", emoteHandler.ServeImage)
		router.Get("/users/{id}/avatar", userHandler.GetAvatar)
		router.Post("/invite/validate", authHandler.ValidateInviteCode)

		// Authenticated
		router.Group(func(router chi.Router) {
			router.Use(middleware.RequireAuth(authSvc))
			router.Get("/me", authHandler.Me)
			router.Get("/channels", channelHandler.List)
			router.Get("/channels/voice", channelHandler.ListVoice)
			router.Get("/channels/unread", channelHandler.GetUnreadCounts)
			router.Get("/channels/{id}", channelHandler.Get)
			router.Put("/channels/{id}/read", channelHandler.MarkRead)
			router.Get("/search", messageHandler.Search)
			router.Get("/channels/{id}/messages", messageHandler.GetHistory)
			router.Get("/channels/{id}/messages/around", messageHandler.GetMessagesAround)
			router.Get("/channels/{id}/messages/newer", messageHandler.GetNewer)
			router.Get("/channels/{id}/pins", messageHandler.GetPinnedMessages)
			router.Put("/messages/{id}/pin", messageHandler.PinMessage)
			router.Delete("/messages/{id}/pin", messageHandler.UnpinMessage)
			router.Post("/dms", dmHandler.CreateOrGet)
			router.Get("/dms", dmHandler.List)
			router.Get("/dms/{id}/messages", dmHandler.GetHistory)
			router.Get("/dms/{id}/pins", dmHandler.GetPins)
			router.Get("/users", userHandler.List)
			router.Put("/users/me/username", authHandler.SetUsername)
			router.Put("/users/me/display-name", userHandler.UpdateDisplayName)
			router.Put("/users/me/color", userHandler.UpdateColor)
			router.Post("/users/me/avatar", userHandler.UploadAvatar)
			router.Get("/emotes", emoteHandler.List)
			router.Get("/export", exportHandler.Export)
			if mediaHandler != nil {
				router.Post("/upload/image", mediaHandler.UploadImage)
				router.Post("/upload/video", mediaHandler.UploadVideo)
			}

			// Admin only
			router.Group(func(router chi.Router) {
				router.Use(middleware.RequireAdmin)
				router.Post("/channels", channelHandler.Create)
				router.Put("/channels/{id}", channelHandler.Update)
				router.Delete("/channels/{id}", channelHandler.Delete)
				router.Post("/emotes", emoteHandler.Create)
				router.Delete("/emotes/{id}", emoteHandler.Delete)

				router.Get("/admin/channels", channelHandler.ListAll)
				router.Route("/admin", func(router chi.Router) {
					router.Get("/users", adminHandler.ListUsers)
					router.Put("/users/{id}/admin", adminHandler.SetAdmin)
					router.Put("/users/{id}/ban", adminHandler.BanUser)
					router.Delete("/users/{id}/messages", adminHandler.DeleteUserMessages)
					router.Delete("/users/{id}", adminHandler.DeleteUser)
					router.Get("/stats", adminHandler.GetStats)
					router.Post("/messages/cleanup", adminHandler.CleanupMessages)
					router.Get("/settings", adminHandler.GetSettings)
					router.Put("/settings", adminHandler.UpdateSettings)
					router.Get("/media", adminHandler.ListMedia)
					router.Get("/media/deleted", adminHandler.ListDeletedMedia)
					router.Get("/media/stats", adminHandler.GetMediaStats)
					router.Delete("/media/{id}", adminHandler.DeleteMedia)
					router.Post("/media/bulk-delete", adminHandler.BulkDeleteMedia)
					router.Post("/invite-codes", adminHandler.CreateInviteCode)
					router.Get("/invite-codes", adminHandler.ListInviteCodes)
					router.Delete("/invite-codes/{id}", adminHandler.DeleteInviteCode)
				})
			})
		})

		// WebSocket (auth via first message)
		router.Get("/ws", ws.ServeWS(hub, authSvc, messageSvc, dmSvc, allowedOrigins))
	})

	// SPA static files — serve real files directly, fall back to index.html
	router.Handle("/*", spaHandler(staticFS))

	return router
}

// spaHandler serves static files from the given FS, falling back to
// index.html for paths that don't match a real file (SPA client routing).
func spaHandler(staticFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		if path != "/" {
			// Strip leading slash for fs.Open
			if _, err := fs.Stat(staticFS, path[1:]); err != nil {
				// File doesn't exist — serve index.html for client-side routing
				request.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(writer, request)
	})
}

// cloudflareRealIP copies CF-Connecting-IP into X-Real-IP so Chi's
// RealIP middleware picks up the actual client address.
func cloudflareRealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if cfIP := request.Header.Get("CF-Connecting-IP"); cfIP != "" {
			request.Header.Set("X-Real-IP", cfIP)
		}
		next.ServeHTTP(writer, request)
	})
}
