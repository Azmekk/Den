package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrChannelNotFound  = errors.New("channel not found")
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
	IsVoice   bool      `json:"is_voice"`
	CreatedAt string    `json:"created_at"`
}

func channelInfoFromDB(ch db.Channel) ChannelInfo {
	info := ChannelInfo{
		ID:        ch.ID,
		Name:      ch.Name,
		Position:  ch.Position,
		IsVoice:   ch.IsVoice,
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

func (s *ChannelService) ListVoice(ctx context.Context) ([]ChannelInfo, error) {
	channels, err := s.queries.ListVoiceChannels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ChannelInfo, len(channels))
	for i, ch := range channels {
		result[i] = channelInfoFromDB(ch)
	}
	return result, nil
}

func (s *ChannelService) ListAll(ctx context.Context) ([]ChannelInfo, error) {
	channels, err := s.queries.ListAllChannels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ChannelInfo, len(channels))
	for i, ch := range channels {
		result[i] = channelInfoFromDB(ch)
	}
	return result, nil
}

func (s *ChannelService) Create(ctx context.Context, name, topic string, position int32, isVoice bool) (ChannelInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return ChannelInfo{}, ErrInvalidInput
	}

	ch, err := s.queries.CreateChannel(ctx, db.CreateChannelParams{
		Name:     name,
		Topic:    sql.NullString{String: topic, Valid: topic != ""},
		Position: position,
		IsVoice:  isVoice,
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

type UnreadInfo struct {
	ChannelID    uuid.UUID `json:"channel_id"`
	UnreadCount  int       `json:"unread_count"`
	MentionCount int       `json:"mention_count"`
}

func (s *ChannelService) GetUnreadCounts(ctx context.Context, userID uuid.UUID) ([]UnreadInfo, error) {
	rows, err := s.queries.GetUnreadCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]UnreadInfo, len(rows))
	for i, r := range rows {
		result[i] = UnreadInfo{
			ChannelID:    r.ChannelID,
			UnreadCount:  int(r.UnreadCount),
			MentionCount: int(r.MentionCount),
		}
	}
	return result, nil
}

func (s *ChannelService) MarkChannelRead(ctx context.Context, userID, channelID uuid.UUID) error {
	return s.queries.UpsertChannelRead(ctx, db.UpsertChannelReadParams{
		UserID:    userID,
		ChannelID: channelID,
	})
}
