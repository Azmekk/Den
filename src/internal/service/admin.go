package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/martinmckenna/den/internal/db"
)

var (
	ErrSelfDemotion = errors.New("cannot remove your own admin status")
	ErrSelfDeletion = errors.New("cannot delete your own account")
)

type AdminService struct {
	queries *db.Queries
	authSvc *AuthService
}

func NewAdminService(queries *db.Queries, authSvc *AuthService) *AdminService {
	return &AdminService{queries: queries, authSvc: authSvc}
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
	return map[string]any{
		"open_registration": s.authSvc.IsOpenRegistration(),
		"instance_name":     s.authSvc.GetInstanceName(),
	}
}

func (s *AdminService) UpdateSettings(openRegistration *bool, instanceName *string) {
	if openRegistration != nil {
		s.authSvc.SetOpenRegistration(*openRegistration)
	}
	if instanceName != nil {
		s.authSvc.SetInstanceName(*instanceName)
	}
}
