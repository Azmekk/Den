package router

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/martinmckenna/den/internal/handler"
	"github.com/martinmckenna/den/internal/middleware"
	"github.com/martinmckenna/den/internal/service"
	"github.com/martinmckenna/den/internal/ws"
)

func New(authSvc *service.AuthService, channelSvc *service.ChannelService, messageSvc *service.MessageService, userSvc *service.UserService, adminSvc *service.AdminService, emoteSvc *service.EmoteService, hub *ws.Hub, staticFS fs.FS, bucketConfigured bool) chi.Router {
	authH := handler.NewAuthHandler(authSvc)
	channelH := handler.NewChannelHandler(channelSvc)
	messageH := handler.NewMessageHandler(messageSvc)
	userH := handler.NewUserHandler(userSvc)
	adminH := handler.NewAdminHandler(adminSvc)
	emoteH := handler.NewEmoteHandler(emoteSvc, hub)
	configH := handler.NewConfigHandler(bucketConfigured)

	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))
	r.Use(chimw.Heartbeat("/healthz"))

	r.Route("/api", func(r chi.Router) {
		// Public
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)
		r.Get("/config", configH.GetConfig)
		r.Get("/emotes/{id}/image", emoteH.ServeImage)

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(authSvc))
			r.Get("/me", authH.Me)
			r.Post("/change-password", authH.ChangePassword)
			r.Get("/channels", channelH.List)
			r.Get("/channels/unread", channelH.GetUnreadCounts)
			r.Get("/channels/{id}", channelH.Get)
			r.Put("/channels/{id}/read", channelH.MarkRead)
			r.Get("/channels/{id}/messages", messageH.GetHistory)
			r.Get("/users", userH.List)
			r.Get("/emotes", emoteH.List)

			// Admin only
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/channels", channelH.Create)
				r.Put("/channels/{id}", channelH.Update)
				r.Delete("/channels/{id}", channelH.Delete)
				r.Post("/emotes", emoteH.Create)
				r.Delete("/emotes/{id}", emoteH.Delete)

				r.Route("/admin", func(r chi.Router) {
					r.Get("/users", adminH.ListUsers)
					r.Put("/users/{id}/admin", adminH.SetAdmin)
					r.Post("/users/{id}/reset-password", adminH.ResetPassword)
					r.Delete("/users/{id}", adminH.DeleteUser)
					r.Get("/stats", adminH.GetStats)
					r.Post("/messages/cleanup", adminH.CleanupMessages)
					r.Get("/settings", adminH.GetSettings)
					r.Put("/settings", adminH.UpdateSettings)
				})
			})
		})

		// WebSocket (auth via query param)
		r.Get("/ws", ws.ServeWS(hub, authSvc, messageSvc))
	})

	// SPA static files
	r.Handle("/*", http.FileServer(http.FS(staticFS)))

	return r
}
