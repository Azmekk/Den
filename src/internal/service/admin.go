package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrSelfDemotion = errors.New("cannot remove your own admin status")
	ErrSelfDeletion = errors.New("cannot delete your own account")
	ErrSelfBan      = errors.New("cannot ban yourself")
)

// UserKicker is implemented by the WebSocket hub to disconnect a user.
type UserKicker interface {
	KickUser(userID uuid.UUID)
}

type AdminService struct {
	queries         *db.Queries
	authSvc         *AuthService
	hub             UserKicker
	mu              sync.RWMutex
	maxMessages     int64
	maxMessageChars int
}

func NewAdminService(queries *db.Queries, authSvc *AuthService, hub UserKicker) *AdminService {
	return &AdminService{
		queries:         queries,
		authSvc:         authSvc,
		hub:             hub,
		maxMessages:     100000,
		maxMessageChars: 2000,
	}
}

func (service *AdminService) LoadSettings(ctx context.Context) error {
	row, fetchError := service.queries.GetAdminSettings(ctx)
	if fetchError != nil {
		return fetchError
	}
	service.mu.Lock()
	service.maxMessages = int64(row.MaxMessages)
	service.maxMessageChars = int(row.MaxMessageChars)
	service.mu.Unlock()
	service.authSvc.SetOpenRegistration(row.OpenRegistration)
	service.authSvc.SetInstanceName(row.InstanceName)
	return nil
}

func (service *AdminService) GetMaxMessageChars() int {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.maxMessageChars
}

func (service *AdminService) ListUsers(ctx context.Context) ([]PublicUserInfo, error) {
	rows, fetchError := service.queries.ListUsers(ctx)
	if fetchError != nil {
		return nil, fetchError
	}
	users := make([]PublicUserInfo, len(rows))
	for index, row := range rows {
		userInfo := PublicUserInfo{
			ID:       row.ID,
			Username: row.Username,
			IsAdmin:  row.IsAdmin,
			Banned:   row.Banned,
		}
		if row.DisplayName.Valid {
			userInfo.DisplayName = row.DisplayName.String
		}
		if row.AvatarUrl.Valid {
			userInfo.AvatarURL = row.AvatarUrl.String
		}
		users[index] = userInfo
	}
	return users, nil
}

func (service *AdminService) SetAdmin(ctx context.Context, callerID, targetID uuid.UUID, isAdmin bool) error {
	if callerID == targetID && !isAdmin {
		return ErrSelfDemotion
	}
	return service.queries.SetUserAdmin(ctx, db.SetUserAdminParams{
		ID:      targetID,
		IsAdmin: isAdmin,
	})
}

func (service *AdminService) DeleteUser(ctx context.Context, callerID, targetID uuid.UUID) error {
	if callerID == targetID {
		return ErrSelfDeletion
	}

	// Delete from DB (CASCADE will clean up refresh_tokens)
	if deleteError := service.queries.DeleteUser(ctx, targetID); deleteError != nil {
		return deleteError
	}

	service.authSvc.InvalidateUserCache(targetID)

	// Kick from WebSocket
	service.hub.KickUser(targetID)

	return nil
}

func (service *AdminService) BanUser(ctx context.Context, callerID, targetID uuid.UUID, ban bool) error {
	if callerID == targetID {
		return ErrSelfBan
	}

	// Set local banned flag
	if banError := service.queries.SetUserBanned(ctx, db.SetUserBannedParams{
		ID:     targetID,
		Banned: ban,
	}); banError != nil {
		return banError
	}

	service.authSvc.InvalidateUserCache(targetID)

	if ban {
		// Revoke all refresh tokens so the banned user can't get new access tokens
		if revokeError := service.authSvc.RevokeAllUserTokens(ctx, targetID); revokeError != nil {
			log.Printf("warning: failed to revoke refresh tokens for banned user: %v", revokeError)
		}
		// Kick from WebSocket immediately
		service.hub.KickUser(targetID)
	}

	return nil
}

func (service *AdminService) DeleteUserMessages(ctx context.Context, userID uuid.UUID) (int64, error) {
	return service.queries.DeleteMessagesByUserID(ctx, userID)
}

func (service *AdminService) GetStats(ctx context.Context) (map[string]int64, error) {
	msgCount, msgError := service.queries.CountMessages(ctx)
	if msgError != nil {
		return nil, msgError
	}
	userCount, userError := service.queries.CountUsers(ctx)
	if userError != nil {
		return nil, userError
	}
	chanCount, chanError := service.queries.CountChannels(ctx)
	if chanError != nil {
		return nil, chanError
	}
	return map[string]int64{
		"message_count": msgCount,
		"user_count":    userCount,
		"channel_count": chanCount,
	}, nil
}

func (service *AdminService) DeleteOldestMessages(ctx context.Context, count int32) error {
	return service.queries.DeleteOldestMessages(ctx, count)
}

func (service *AdminService) GetSettings() map[string]any {
	service.mu.RLock()
	maxMsg := service.maxMessages
	maxChars := service.maxMessageChars
	service.mu.RUnlock()
	return map[string]any{
		"open_registration": service.authSvc.IsOpenRegistration(),
		"instance_name":     service.authSvc.GetInstanceName(),
		"max_messages":      maxMsg,
		"max_message_chars": maxChars,
	}
}

func (service *AdminService) RunMessageCleanupLoop(ctx context.Context, batchSize int32, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			service.mu.RLock()
			maxMessages := service.maxMessages
			service.mu.RUnlock()
			if maxMessages <= 0 {
				continue
			}
			count, countError := service.queries.CountMessages(ctx)
			if countError != nil {
				log.Printf("message cleanup: count error: %v", countError)
				continue
			}
			if count > maxMessages {
				toDelete := int32(count-maxMessages) + batchSize/2
				if toDelete > 0 {
					_ = service.queries.DeleteOldestMessages(ctx, toDelete)
					log.Printf("message cleanup: deleted %d oldest unpinned messages", toDelete)
				}
			}
		}
	}
}

type MediaUploadInfo struct {
	ID               uuid.UUID  `json:"id"`
	UploaderID       uuid.UUID  `json:"uploader_id"`
	UploaderUsername string     `json:"uploader_username"`
	BucketKey        string     `json:"bucket_key"`
	MediaType        string     `json:"media_type"`
	FileSize         int64      `json:"file_size"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type MediaTypeStats struct {
	MediaType string `json:"media_type"`
	Count     int64  `json:"count"`
	TotalSize int64  `json:"total_size"`
}

type MediaStats struct {
	TotalCount int64            `json:"total_count"`
	TotalSize  int64            `json:"total_size"`
	ByType     []MediaTypeStats `json:"by_type"`
}

type PaginatedMedia struct {
	Items      []MediaUploadInfo `json:"items"`
	TotalCount int64             `json:"total_count"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
}

func (service *AdminService) ListMedia(ctx context.Context, page, pageSize int) (PaginatedMedia, error) {
	offset := int32((page - 1) * pageSize)
	rows, fetchError := service.queries.ListActiveMediaUploads(ctx, db.ListActiveMediaUploadsParams{
		Limit:  int32(pageSize),
		Offset: offset,
	})
	if fetchError != nil {
		return PaginatedMedia{}, fetchError
	}
	total, countError := service.queries.CountActiveMediaUploads(ctx)
	if countError != nil {
		return PaginatedMedia{}, countError
	}
	items := make([]MediaUploadInfo, len(rows))
	for index, row := range rows {
		items[index] = MediaUploadInfo{
			ID:               row.ID,
			UploaderID:       row.UploaderID,
			UploaderUsername: row.UploaderUsername,
			BucketKey:        row.BucketKey,
			MediaType:        row.MediaType,
			FileSize:         row.FileSize,
			ExpiresAt:        row.ExpiresAt,
			CreatedAt:        row.CreatedAt,
		}
	}
	return PaginatedMedia{Items: items, TotalCount: total, Page: page, PageSize: pageSize}, nil
}

func (service *AdminService) ListDeletedMedia(ctx context.Context, page, pageSize int) (PaginatedMedia, error) {
	offset := int32((page - 1) * pageSize)
	rows, fetchError := service.queries.ListDeletedMediaUploads(ctx, db.ListDeletedMediaUploadsParams{
		Limit:  int32(pageSize),
		Offset: offset,
	})
	if fetchError != nil {
		return PaginatedMedia{}, fetchError
	}
	total, countError := service.queries.CountDeletedMediaUploads(ctx)
	if countError != nil {
		return PaginatedMedia{}, countError
	}
	items := make([]MediaUploadInfo, len(rows))
	for index, row := range rows {
		var deletedAt *time.Time
		if row.DeletedAt.Valid {
			deletedAt = &row.DeletedAt.Time
		}
		items[index] = MediaUploadInfo{
			ID:               row.ID,
			UploaderID:       row.UploaderID,
			UploaderUsername: row.UploaderUsername,
			BucketKey:        row.BucketKey,
			MediaType:        row.MediaType,
			FileSize:         row.FileSize,
			ExpiresAt:        row.ExpiresAt,
			CreatedAt:        row.CreatedAt,
			DeletedAt:        deletedAt,
		}
	}
	return PaginatedMedia{Items: items, TotalCount: total, Page: page, PageSize: pageSize}, nil
}

func (service *AdminService) GetMediaStats(ctx context.Context) (MediaStats, error) {
	totals, fetchError := service.queries.GetMediaStats(ctx)
	if fetchError != nil {
		return MediaStats{}, fetchError
	}
	byType, typeError := service.queries.GetMediaStatsByType(ctx)
	if typeError != nil {
		return MediaStats{}, typeError
	}
	typeStats := make([]MediaTypeStats, len(byType))
	for index, typeStat := range byType {
		typeStats[index] = MediaTypeStats{
			MediaType: typeStat.MediaType,
			Count:     typeStat.Count,
			TotalSize: typeStat.TotalSize,
		}
	}
	return MediaStats{
		TotalCount: totals.TotalCount,
		TotalSize:  totals.TotalSize,
		ByType:     typeStats,
	}, nil
}

// InviteCodeInfo is the JSON-serializable representation of an invite code.
type InviteCodeInfo struct {
	ID                uuid.UUID  `json:"id"`
	Code              string     `json:"code"`
	MaxUses           *int32     `json:"max_uses"`
	UseCount          int32      `json:"use_count"`
	ExpiresAt         *time.Time `json:"expires_at"`
	CreatedBy         uuid.UUID  `json:"created_by"`
	CreatedByUsername string     `json:"created_by_username"`
	CreatedAt         time.Time  `json:"created_at"`
}

func (service *AdminService) CreateInviteCode(ctx context.Context, code string, maxUses sql.NullInt32, expiresAt sql.NullTime, createdBy uuid.UUID) (InviteCodeInfo, error) {
	inviteCode, createError := service.queries.CreateInviteCode(ctx, db.CreateInviteCodeParams{
		Code:      code,
		MaxUses:   maxUses,
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
	})
	if createError != nil {
		return InviteCodeInfo{}, createError
	}
	info := InviteCodeInfo{
		ID:        inviteCode.ID,
		Code:      inviteCode.Code,
		UseCount:  inviteCode.UseCount,
		CreatedBy: inviteCode.CreatedBy,
		CreatedAt: inviteCode.CreatedAt,
	}
	if inviteCode.MaxUses.Valid {
		info.MaxUses = &inviteCode.MaxUses.Int32
	}
	if inviteCode.ExpiresAt.Valid {
		info.ExpiresAt = &inviteCode.ExpiresAt.Time
	}
	return info, nil
}

func (service *AdminService) ListInviteCodes(ctx context.Context) ([]InviteCodeInfo, error) {
	rows, fetchError := service.queries.ListInviteCodes(ctx)
	if fetchError != nil {
		return nil, fetchError
	}
	codes := make([]InviteCodeInfo, len(rows))
	for index, row := range rows {
		info := InviteCodeInfo{
			ID:                row.ID,
			Code:              row.Code,
			UseCount:          row.UseCount,
			CreatedBy:         row.CreatedBy,
			CreatedByUsername: row.CreatedByUsername,
			CreatedAt:         row.CreatedAt,
		}
		if row.MaxUses.Valid {
			info.MaxUses = &row.MaxUses.Int32
		}
		if row.ExpiresAt.Valid {
			info.ExpiresAt = &row.ExpiresAt.Time
		}
		codes[index] = info
	}
	return codes, nil
}

func (service *AdminService) DeleteInviteCode(ctx context.Context, codeID uuid.UUID) error {
	return service.queries.DeleteInviteCode(ctx, codeID)
}

func (service *AdminService) UpdateSettings(ctx context.Context, openRegistration *bool, instanceName *string, maxMessages *int64, maxMessageChars *int) error {
	current := service.GetSettings()
	openReg := current["open_registration"].(bool)
	instName := current["instance_name"].(string)
	maxMsg := current["max_messages"].(int64)
	maxChars := current["max_message_chars"].(int)

	if openRegistration != nil {
		openReg = *openRegistration
	}
	if instanceName != nil {
		instName = *instanceName
	}
	if maxMessages != nil {
		maxMsg = *maxMessages
	}
	if maxMessageChars != nil {
		maxChars = *maxMessageChars
	}

	updateError := service.queries.UpdateAdminSettings(ctx, db.UpdateAdminSettingsParams{
		OpenRegistration: openReg,
		InstanceName:     instName,
		MaxMessages:      int32(maxMsg),
		MaxMessageChars:  int32(maxChars),
	})
	if updateError != nil {
		return updateError
	}

	service.mu.Lock()
	service.maxMessages = maxMsg
	service.maxMessageChars = maxChars
	service.mu.Unlock()
	service.authSvc.SetOpenRegistration(openReg)
	service.authSvc.SetInstanceName(instName)
	return nil
}
