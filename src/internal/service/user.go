package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type UserService struct {
	queries *db.Queries
}

func NewUserService(queries *db.Queries) *UserService {
	return &UserService{queries: queries}
}

func (s *UserService) Queries() *db.Queries {
	return s.queries
}

func (s *UserService) GetAvatarURL(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if !user.AvatarUrl.Valid || user.AvatarUrl.String == "" {
		return "", fmt.Errorf("no avatar")
	}
	return user.AvatarUrl.String, nil
}

type PublicUserInfo struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Color       string    `json:"color,omitempty"`
	IsAdmin     bool      `json:"is_admin"`
}

func (s *UserService) List(ctx context.Context) ([]PublicUserInfo, error) {
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
		if row.Color.Valid {
			u.Color = row.Color.String
		}
		users[i] = u
	}
	return users, nil
}

func (s *UserService) UpdateDisplayName(ctx context.Context, userID uuid.UUID, displayName string) (PublicUserInfo, error) {
	if len(displayName) > 64 {
		return PublicUserInfo{}, fmt.Errorf("%w: display name too long", ErrInvalidInput)
	}

	user, err := s.queries.UpdateUserDisplayName(ctx, db.UpdateUserDisplayNameParams{
		ID:          userID,
		DisplayName: sql.NullString{String: displayName, Valid: displayName != ""},
	})
	if err != nil {
		return PublicUserInfo{}, err
	}

	return publicUserInfoFromDB(user), nil
}

func (s *UserService) UpdateColor(ctx context.Context, userID uuid.UUID, color string) (PublicUserInfo, error) {
	if color != "" && !hexColorPattern.MatchString(color) {
		return PublicUserInfo{}, fmt.Errorf("%w: invalid color format, must be #xxxxxx", ErrInvalidInput)
	}

	user, err := s.queries.UpdateUserColor(ctx, db.UpdateUserColorParams{
		ID:    userID,
		Color: sql.NullString{String: color, Valid: color != ""},
	})
	if err != nil {
		return PublicUserInfo{}, err
	}

	return publicUserInfoFromDB(user), nil
}

func publicUserInfoFromDB(u db.User) PublicUserInfo {
	info := PublicUserInfo{
		ID:       u.ID,
		Username: u.Username,
		IsAdmin:  u.IsAdmin,
	}
	if u.DisplayName.Valid {
		info.DisplayName = u.DisplayName.String
	}
	if u.AvatarUrl.Valid {
		info.AvatarURL = u.AvatarUrl.String
	}
	if u.Color.Valid {
		info.Color = u.Color.String
	}
	return info
}
