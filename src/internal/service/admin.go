package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrSelfDemotion    = errors.New("cannot remove your own admin status")
	ErrSelfDeletion    = errors.New("cannot delete your own account")
	ErrInvalidInviteCode = errors.New("invalid or expired invite code")
)

type AdminService struct {
	queries         *db.Queries
	authSvc         *AuthService
	mu              sync.RWMutex
	maxMessages     int64
	maxMessageChars int
}

func NewAdminService(queries *db.Queries, authSvc *AuthService) *AdminService {
	return &AdminService{
		queries:         queries,
		authSvc:         authSvc,
		maxMessages:     100000,
		maxMessageChars: 2000,
	}
}

func (s *AdminService) LoadSettings(ctx context.Context) error {
	row, err := s.queries.GetAdminSettings(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.maxMessages = int64(row.MaxMessages)
	s.maxMessageChars = int(row.MaxMessageChars)
	s.mu.Unlock()
	s.authSvc.SetOpenRegistration(row.OpenRegistration)
	s.authSvc.SetInstanceName(row.InstanceName)
	return nil
}

func (s *AdminService) GetMaxMessageChars() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxMessageChars
}

func (s *AdminService) ListUsers(ctx context.Context) ([]PublicUserInfo, error) {
	rows, err := s.queries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]PublicUserInfo, len(rows))
	for i, row := range rows {
		u := PublicUserInfo{
			ID:       row.ID,
			Username: row.Username,
			IsAdmin:  row.IsAdmin,
		}
		if row.DisplayName.Valid {
			u.DisplayName = row.DisplayName.String
		}
		if row.AvatarUrl.Valid {
			u.AvatarURL = row.AvatarUrl.String
		}
		users[i] = u
	}
	return users, nil
}

func (s *AdminService) SetAdmin(ctx context.Context, callerID, targetID uuid.UUID, isAdmin bool) error {
	if callerID == targetID && !isAdmin {
		return ErrSelfDemotion
	}
	return s.queries.SetUserAdmin(ctx, db.SetUserAdminParams{
		ID:      targetID,
		IsAdmin: isAdmin,
	})
}

func (s *AdminService) ResetPassword(ctx context.Context, userID uuid.UUID) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tempPassword := hex.EncodeToString(raw)

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	if err := s.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: string(hash),
	}); err != nil {
		return "", err
	}

	_ = s.queries.DeleteRefreshTokensByUser(ctx, userID)

	return tempPassword, nil
}

func (s *AdminService) DeleteUser(ctx context.Context, callerID, targetID uuid.UUID) error {
	if callerID == targetID {
		return ErrSelfDeletion
	}
	return s.queries.DeleteUser(ctx, targetID)
}

func (s *AdminService) GetStats(ctx context.Context) (map[string]int64, error) {
	msgCount, err := s.queries.CountMessages(ctx)
	if err != nil {
		return nil, err
	}
	userCount, err := s.queries.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	chanCount, err := s.queries.CountChannels(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]int64{
		"message_count": msgCount,
		"user_count":    userCount,
		"channel_count": chanCount,
	}, nil
}

func (s *AdminService) DeleteOldestMessages(ctx context.Context, count int32) error {
	return s.queries.DeleteOldestMessages(ctx, count)
}

func (s *AdminService) GetSettings() map[string]any {
	s.mu.RLock()
	maxMsg := s.maxMessages
	maxChars := s.maxMessageChars
	s.mu.RUnlock()
	return map[string]any{
		"open_registration": s.authSvc.IsOpenRegistration(),
		"instance_name":     s.authSvc.GetInstanceName(),
		"max_messages":      maxMsg,
		"max_message_chars": maxChars,
	}
}

func (s *AdminService) RunMessageCleanupLoop(ctx context.Context, batchSize int32, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			maxMessages := s.maxMessages
			s.mu.RUnlock()
			if maxMessages <= 0 {
				continue
			}
			count, err := s.queries.CountMessages(ctx)
			if err != nil {
				log.Printf("message cleanup: count error: %v", err)
				continue
			}
			if count > maxMessages {
				toDelete := int32(count-maxMessages) + batchSize/2
				if toDelete > 0 {
					_ = s.queries.DeleteOldestMessages(ctx, toDelete)
					log.Printf("message cleanup: deleted %d oldest unpinned messages", toDelete)
				}
			}
		}
	}
}

type MediaUploadInfo struct {
	ID               uuid.UUID `json:"id"`
	UploaderID       uuid.UUID `json:"uploader_id"`
	UploaderUsername string    `json:"uploader_username"`
	BucketKey        string    `json:"bucket_key"`
	MediaType        string    `json:"media_type"`
	FileSize         int64     `json:"file_size"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
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

func (s *AdminService) ListMedia(ctx context.Context) ([]MediaUploadInfo, error) {
	rows, err := s.queries.ListAllMediaUploads(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]MediaUploadInfo, len(rows))
	for i, row := range rows {
		result[i] = MediaUploadInfo{
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
	return result, nil
}

func (s *AdminService) GetMediaStats(ctx context.Context) (MediaStats, error) {
	totals, err := s.queries.GetMediaStats(ctx)
	if err != nil {
		return MediaStats{}, err
	}
	byType, err := s.queries.GetMediaStatsByType(ctx)
	if err != nil {
		return MediaStats{}, err
	}
	typeStats := make([]MediaTypeStats, len(byType))
	for i, t := range byType {
		typeStats[i] = MediaTypeStats{
			MediaType: t.MediaType,
			Count:     t.Count,
			TotalSize: t.TotalSize,
		}
	}
	return MediaStats{
		TotalCount: totals.TotalCount,
		TotalSize:  totals.TotalSize,
		ByType:     typeStats,
	}, nil
}

type InviteCodeInfo struct {
	ID               uuid.UUID  `json:"id"`
	Code             string     `json:"code"`
	MaxUses          *int32     `json:"max_uses"`
	UseCount         int32      `json:"use_count"`
	ExpiresAt        *time.Time `json:"expires_at"`
	CreatedBy        uuid.UUID  `json:"created_by"`
	CreatedByUsername string    `json:"created_by_username"`
	CreatedAt        time.Time  `json:"created_at"`
}

func (s *AdminService) CreateInviteCode(ctx context.Context, createdBy uuid.UUID, maxUses *int32, expiresAt *time.Time) (InviteCodeInfo, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return InviteCodeInfo{}, err
	}
	code := hex.EncodeToString(raw)

	params := db.CreateInviteCodeParams{
		Code:      code,
		CreatedBy: createdBy,
	}
	if maxUses != nil {
		params.MaxUses.Int32 = *maxUses
		params.MaxUses.Valid = true
	}
	if expiresAt != nil {
		params.ExpiresAt.Time = *expiresAt
		params.ExpiresAt.Valid = true
	}

	row, err := s.queries.CreateInviteCode(ctx, params)
	if err != nil {
		return InviteCodeInfo{}, err
	}

	info := InviteCodeInfo{
		ID:        row.ID,
		Code:      row.Code,
		UseCount:  row.UseCount,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
	}
	if row.MaxUses.Valid {
		info.MaxUses = &row.MaxUses.Int32
	}
	if row.ExpiresAt.Valid {
		info.ExpiresAt = &row.ExpiresAt.Time
	}
	return info, nil
}

func (s *AdminService) ListInviteCodes(ctx context.Context) ([]InviteCodeInfo, error) {
	rows, err := s.queries.ListInviteCodes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]InviteCodeInfo, len(rows))
	for i, row := range rows {
		info := InviteCodeInfo{
			ID:               row.ID,
			Code:             row.Code,
			UseCount:         row.UseCount,
			CreatedBy:        row.CreatedBy,
			CreatedByUsername: row.CreatedByUsername,
			CreatedAt:        row.CreatedAt,
		}
		if row.MaxUses.Valid {
			info.MaxUses = &row.MaxUses.Int32
		}
		if row.ExpiresAt.Valid {
			info.ExpiresAt = &row.ExpiresAt.Time
		}
		result[i] = info
	}
	return result, nil
}

func (s *AdminService) DeleteInviteCode(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteInviteCode(ctx, id)
}

func (s *AdminService) ValidateAndUseInviteCode(ctx context.Context, code string) error {
	ic, err := s.queries.GetInviteCodeByCode(ctx, code)
	if err != nil {
		return ErrInvalidInviteCode
	}
	if ic.ExpiresAt.Valid && time.Now().After(ic.ExpiresAt.Time) {
		return ErrInvalidInviteCode
	}
	if ic.MaxUses.Valid && ic.UseCount >= ic.MaxUses.Int32 {
		return ErrInvalidInviteCode
	}
	return s.queries.IncrementInviteCodeUseCount(ctx, ic.ID)
}

func (s *AdminService) UpdateSettings(ctx context.Context, openRegistration *bool, instanceName *string, maxMessages *int64, maxMessageChars *int) error {
	// Read current values
	current := s.GetSettings()
	or := current["open_registration"].(bool)
	in := current["instance_name"].(string)
	mm := current["max_messages"].(int64)
	mc := current["max_message_chars"].(int)

	if openRegistration != nil {
		or = *openRegistration
	}
	if instanceName != nil {
		in = *instanceName
	}
	if maxMessages != nil {
		mm = *maxMessages
	}
	if maxMessageChars != nil {
		mc = *maxMessageChars
	}

	err := s.queries.UpdateAdminSettings(ctx, db.UpdateAdminSettingsParams{
		OpenRegistration: or,
		InstanceName:     in,
		MaxMessages:      int32(mm),
		MaxMessageChars:  int32(mc),
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.maxMessages = mm
	s.maxMessageChars = mc
	s.mu.Unlock()
	s.authSvc.SetOpenRegistration(or)
	s.authSvc.SetInstanceName(in)
	return nil
}
