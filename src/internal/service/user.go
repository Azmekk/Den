package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

type UserService struct {
	queries *db.Queries
}

func NewUserService(queries *db.Queries) *UserService {
	return &UserService{queries: queries}
}

type PublicUserInfo struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
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
		users[i] = u
	}
	return users, nil
}
