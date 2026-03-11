package router

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/Azmekk/den/internal/handler"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
	"github.com/Azmekk/den/internal/ws"
)

func New(authSvc *service.AuthService, channelSvc *service.ChannelService, messageSvc *service.MessageService, userSvc *service.UserService, adminSvc *service.AdminService, emoteSvc *service.EmoteService, dmSvc *service.DMService, mediaSvc *service.MediaService, voiceSvc *service.VoiceService, unfurlSvc *service.UnfurlService, hub *ws.Hub, staticFS fs.FS, bucketConfigured bool) chi.Router {
	authH := handler.NewAuthHandler(authSvc, hub)
	channelH := handler.NewChannelHandler(channelSvc)
	messageH := handler.NewMessageHandler(messageSvc, hub)
	userH := handler.NewUserHandler(userSvc, mediaSvc, hub)
	adminH := handler.NewAdminHandler(adminSvc, mediaSvc)
	emoteH := handler.NewEmoteHandler(emoteSvc, hub)
	configH := handler.NewConfigHandler(bucketConfigured, voiceSvc != nil, adminSvc.GetMaxMessageChars, authSvc.IsOpenRegistration)
	dmH := handler.NewDMHandler(dmSvc)
	var mediaH *handler.MediaHandler
	if mediaSvc != nil {
		mediaH = handler.NewMediaHandler(mediaSvc)
	}
	unfurlH := handler.NewUnfurlHandler(unfurlSvc)
	var voiceH *handler.VoiceHandler
	if voiceSvc != nil {
		voiceH = handler.NewVoiceHandler(voiceSvc, channelSvc)
	}

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
		r.Get("/users/{id}/avatar", userH.GetAvatar)

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(authSvc))
			r.Get("/me", authH.Me)
			r.Post("/change-password", authH.ChangePassword)
			r.Get("/channels", channelH.List)
			r.Get("/channels/voice", channelH.ListVoice)
			r.Get("/channels/unread", channelH.GetUnreadCounts)
			r.Get("/channels/{id}", channelH.Get)
			r.Put("/channels/{id}/read", channelH.MarkRead)
			r.Get("/search", messageH.Search)
			r.Get("/channels/{id}/messages", messageH.GetHistory)
			r.Get("/channels/{id}/messages/around", messageH.GetMessagesAround)
			r.Get("/channels/{id}/messages/newer", messageH.GetNewer)
			r.Get("/channels/{id}/pins", messageH.GetPinnedMessages)
			r.Put("/messages/{id}/pin", messageH.PinMessage)
			r.Delete("/messages/{id}/pin", messageH.UnpinMessage)
			r.Post("/dms", dmH.CreateOrGet)
			r.Get("/dms", dmH.List)
			r.Get("/dms/{id}/messages", dmH.GetHistory)
			r.Get("/dms/{id}/pins", dmH.GetPins)
			r.Get("/users", userH.List)
			r.Put("/users/me/display-name", userH.UpdateDisplayName)
			r.Put("/users/me/color", userH.UpdateColor)
			r.Post("/users/me/avatar", userH.UploadAvatar)
			r.Get("/emotes", emoteH.List)
			r.Get("/unfurl", unfurlH.Unfurl)
			if mediaH != nil {
				r.Post("/upload/image", mediaH.UploadImage)
				r.Post("/upload/video", mediaH.UploadVideo)
			}
			if voiceH != nil {
				r.Post("/voice/{channelId}/join", voiceH.Join)
			}

			// Admin only
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/channels", channelH.Create)
				r.Put("/channels/{id}", channelH.Update)
				r.Delete("/channels/{id}", channelH.Delete)
				r.Post("/emotes", emoteH.Create)
				r.Delete("/emotes/{id}", emoteH.Delete)

				r.Get("/admin/channels", channelH.ListAll)
				r.Route("/admin", func(r chi.Router) {
					r.Get("/users", adminH.ListUsers)
					r.Put("/users/{id}/admin", adminH.SetAdmin)
					r.Post("/users/{id}/reset-password", adminH.ResetPassword)
					r.Delete("/users/{id}", adminH.DeleteUser)
					r.Get("/stats", adminH.GetStats)
					r.Post("/messages/cleanup", adminH.CleanupMessages)
					r.Get("/settings", adminH.GetSettings)
					r.Put("/settings", adminH.UpdateSettings)
					r.Get("/media", adminH.ListMedia)
					r.Get("/media/stats", adminH.GetMediaStats)
					r.Delete("/media/{id}", adminH.DeleteMedia)
					r.Post("/media/bulk-delete", adminH.BulkDeleteMedia)
					r.Get("/invite-codes", adminH.ListInviteCodes)
					r.Post("/invite-codes", adminH.CreateInviteCode)
					r.Delete("/invite-codes/{id}", adminH.DeleteInviteCode)
				})
			})
		})

		// WebSocket (auth via query param)
		r.Get("/ws", ws.ServeWS(hub, authSvc, messageSvc, dmSvc))
	})

	// SPA static files
	r.Handle("/*", http.FileServer(http.FS(staticFS)))

	return r
}
