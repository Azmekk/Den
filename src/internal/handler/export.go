package handler

import (
	"compress/gzip"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
	"github.com/Azmekk/den/internal/httputil"
	"github.com/Azmekk/den/internal/middleware"
	"github.com/Azmekk/den/internal/service"
)

type ExportHandler struct {
	channelSvc      *service.ChannelService
	userSvc         *service.UserService
	emoteSvc        *service.EmoteService
	dmSvc           *service.DMService
	queries         *db.Queries
	getInstanceName func() string
}

func NewExportHandler(
	channelSvc *service.ChannelService,
	userSvc *service.UserService,
	emoteSvc *service.EmoteService,
	dmSvc *service.DMService,
	queries *db.Queries,
	getInstanceName func() string,
) *ExportHandler {
	return &ExportHandler{
		channelSvc:      channelSvc,
		userSvc:         userSvc,
		emoteSvc:        emoteSvc,
		dmSvc:           dmSvc,
		queries:         queries,
		getInstanceName: getInstanceName,
	}
}

type exportData struct {
	Version         int                      `json:"version"`
	InstanceName    string                   `json:"instance_name"`
	ExportedAt      string                   `json:"exported_at"`
	ExportedBy      uuid.UUID                `json:"exported_by"`
	Users           []service.PublicUserInfo  `json:"users"`
	Emotes          []service.EmoteInfo      `json:"emotes"`
	Channels        []exportChannel          `json:"channels"`
	DMConversations []exportDM               `json:"dm_conversations"`
}

type exportChannel struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Topic     string          `json:"topic,omitempty"`
	Position  int32           `json:"position"`
	IsVoice   bool            `json:"is_voice"`
	CreatedAt string          `json:"created_at"`
	Messages  []exportMessage `json:"messages"`
}

type exportDM struct {
	ID            uuid.UUID       `json:"id"`
	OtherUserID   uuid.UUID       `json:"other_user_id"`
	OtherUsername  string          `json:"other_username"`
	CreatedAt     string          `json:"created_at"`
	Messages      []exportMessage `json:"messages"`
}

type exportMessage struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Content   string    `json:"content"`
	Pinned    bool      `json:"pinned"`
	EditedAt  string    `json:"edited_at,omitempty"`
	CreatedAt string    `json:"created_at"`
}

func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	ctx := r.Context()

	users, err := h.userSvc.List(ctx)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	emotes, err := h.emoteSvc.List(ctx)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch emotes")
		return
	}

	channels, err := h.channelSvc.ListAll(ctx)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch channels")
		return
	}

	exportChannels := make([]exportChannel, 0, len(channels))
	for _, ch := range channels {
		nullID := uuid.NullUUID{UUID: ch.ID, Valid: true}
		rows, err := h.queries.GetAllChannelMessages(ctx, nullID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch channel messages")
			return
		}

		msgs := make([]exportMessage, len(rows))
		for i, row := range rows {
			msgs[i] = exportMessage{
				ID:        row.ID,
				UserID:    row.UserID,
				Content:   row.Content,
				Pinned:    row.Pinned,
				CreatedAt: row.CreatedAt.Format(time.RFC3339),
			}
			if row.EditedAt.Valid {
				msgs[i].EditedAt = row.EditedAt.Time.Format(time.RFC3339)
			}
		}

		exportChannels = append(exportChannels, exportChannel{
			ID:        ch.ID,
			Name:      ch.Name,
			Topic:     ch.Topic,
			Position:  ch.Position,
			IsVoice:   ch.IsVoice,
			CreatedAt: ch.CreatedAt,
			Messages:  msgs,
		})
	}

	dmPairs, err := h.dmSvc.ListConversations(ctx, userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch dm conversations")
		return
	}

	exportDMs := make([]exportDM, 0, len(dmPairs))
	for _, dm := range dmPairs {
		nullID := uuid.NullUUID{UUID: dm.ID, Valid: true}
		rows, err := h.queries.GetAllDMMessages(ctx, nullID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch dm messages")
			return
		}

		msgs := make([]exportMessage, len(rows))
		for i, row := range rows {
			msgs[i] = exportMessage{
				ID:        row.ID,
				UserID:    row.UserID,
				Content:   row.Content,
				Pinned:    row.Pinned,
				CreatedAt: row.CreatedAt.Format(time.RFC3339),
			}
			if row.EditedAt.Valid {
				msgs[i].EditedAt = row.EditedAt.Time.Format(time.RFC3339)
			}
		}

		exportDMs = append(exportDMs, exportDM{
			ID:            dm.ID,
			OtherUserID:   dm.OtherUserID,
			OtherUsername:  dm.OtherUsername,
			CreatedAt:     dm.CreatedAt,
			Messages:      msgs,
		})
	}

	export := exportData{
		Version:         1,
		InstanceName:    h.getInstanceName(),
		ExportedAt:      time.Now().UTC().Format(time.RFC3339),
		ExportedBy:      userID,
		Users:           users,
		Emotes:          emotes,
		Channels:        exportChannels,
		DMConversations: exportDMs,
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="den-export.json.gz"`)

	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to create compressor")
		return
	}
	defer func() {
		if err := gz.Close(); err != nil {
			log.Printf("export: gzip close error: %v", err)
		}
	}()

	enc := json.NewEncoder(gz)
	if err := enc.Encode(export); err != nil {
		log.Printf("export: encoding error: %v", err)
	}
}
