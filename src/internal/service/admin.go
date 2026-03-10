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
	ErrSelfDemotion = errors.New("cannot remove your own admin status")
	ErrSelfDeletion = errors.New("cannot delete your own account")
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
