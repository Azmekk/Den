package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/martinmckenna/den/internal/db"
)

var (
	ErrChannelNotFound = errors.New("channel not found")
	ErrChannelNameTaken = errors.New("channel name already taken")
)

type ChannelService struct {
	queries *db.Queries
}

func NewChannelService(queries *db.Queries) *ChannelService {
	return &ChannelService{queries: queries}
}

type ChannelInfo struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Topic     string    `json:"topic,omitempty"`
	Position  int32     `json:"position"`
	CreatedAt string    `json:"created_at"`
}

func channelInfoFromDB(ch db.Channel) ChannelInfo {
	info := ChannelInfo{
		ID:        ch.ID,
		Name:      ch.Name,
		Position:  ch.Position,
		CreatedAt: ch.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if ch.Topic.Valid {
		info.Topic = ch.Topic.String
	}
	return info
}

func (s *ChannelService) List(ctx context.Context) ([]ChannelInfo, error) {
	channels, err := s.queries.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ChannelInfo, len(channels))
	for i, ch := range channels {
		result[i] = channelInfoFromDB(ch)
	}
	return result, nil
}

func (s *ChannelService) Get(ctx context.Context, id uuid.UUID) (ChannelInfo, error) {
	ch, err := s.queries.GetChannel(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChannelInfo{}, ErrChannelNotFound
		}
		return ChannelInfo{}, err
	}
	return channelInfoFromDB(ch), nil
}

func (s *ChannelService) Create(ctx context.Context, name, topic string, position int32) (ChannelInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return ChannelInfo{}, ErrInvalidInput
	}

	ch, err := s.queries.CreateChannel(ctx, db.CreateChannelParams{
		Name:     name,
		Topic:    sql.NullString{String: topic, Valid: topic != ""},
		Position: position,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return ChannelInfo{}, ErrChannelNameTaken
		}
		return ChannelInfo{}, err
	}
	return channelInfoFromDB(ch), nil
}

func (s *ChannelService) Update(ctx context.Context, id uuid.UUID, name, topic string, position int32) (ChannelInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return ChannelInfo{}, ErrInvalidInput
	}

	ch, err := s.queries.UpdateChannel(ctx, db.UpdateChannelParams{
		ID:       id,
		Name:     name,
		Topic:    sql.NullString{String: topic, Valid: topic != ""},
		Position: position,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChannelInfo{}, ErrChannelNotFound
		}
		if isUniqueViolation(err) {
			return ChannelInfo{}, ErrChannelNameTaken
		}
		return ChannelInfo{}, err
	}
	return channelInfoFromDB(ch), nil
}

func (s *ChannelService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteChannel(ctx, id)
}
