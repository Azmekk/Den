package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/martinmckenna/den/internal/db"
)

var (
	ErrMessageNotFound = errors.New("message not found")
	ErrForbidden       = errors.New("forbidden")
)

type MessageService struct {
	queries *db.Queries
}

func NewMessageService(queries *db.Queries) *MessageService {
	return &MessageService{queries: queries}
}

type MessageInfo struct {
	ID          uuid.UUID `json:"id"`
	ChannelID   uuid.UUID `json:"channel_id"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Content     string    `json:"content"`
	EditedAt    string    `json:"edited_at,omitempty"`
	CreatedAt   string    `json:"created_at"`
}

func messageInfoFromRow(row db.GetLatestMessagesByChannelRow) MessageInfo {
	info := MessageInfo{
		ID:        row.ID,
		ChannelID: row.ChannelID.UUID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.DisplayName.Valid {
		info.DisplayName = row.DisplayName.String
	}
	if row.AvatarUrl.Valid {
		info.AvatarURL = row.AvatarUrl.String
	}
	if row.EditedAt.Valid {
		info.EditedAt = row.EditedAt.Time.Format(time.RFC3339Nano)
	}
	return info
}

func messageInfoFromCursorRow(row db.GetMessagesByChannelRow) MessageInfo {
	info := MessageInfo{
		ID:        row.ID,
		ChannelID: row.ChannelID.UUID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.DisplayName.Valid {
		info.DisplayName = row.DisplayName.String
	}
	if row.AvatarUrl.Valid {
		info.AvatarURL = row.AvatarUrl.String
	}
	if row.EditedAt.Valid {
		info.EditedAt = row.EditedAt.Time.Format(time.RFC3339Nano)
	}
	return info
}

func (s *MessageService) SendMessage(ctx context.Context, channelID, userID uuid.UUID, username, content string) ([]byte, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 2000 {
		return nil, ErrInvalidInput
	}

	msg, err := s.queries.CreateMessage(ctx, db.CreateMessageParams{
		ChannelID: uuid.NullUUID{UUID: channelID, Valid: true},
		UserID:    userID,
		Content:   content,
	})
	if err != nil {
		return nil, err
	}

	envelope := map[string]any{
		"type":       "new_message",
		"id":         msg.ID,
		"channel_id": channelID,
		"user_id":    userID,
		"username":   username,
		"content":    msg.Content,
		"created_at": msg.CreatedAt.Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *MessageService) EditMessage(ctx context.Context, messageID, userID uuid.UUID, content string) ([]byte, uuid.UUID, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 2000 {
		return nil, uuid.Nil, ErrInvalidInput
	}

	existing, err := s.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uuid.Nil, ErrMessageNotFound
		}
		return nil, uuid.Nil, err
	}

	if existing.UserID != userID {
		return nil, uuid.Nil, ErrForbidden
	}

	updated, err := s.queries.UpdateMessageContent(ctx, db.UpdateMessageContentParams{
		ID:      messageID,
		Content: content,
	})
	if err != nil {
		return nil, uuid.Nil, err
	}

	channelID := existing.ChannelID.UUID
	envelope := map[string]any{
		"type":       "edit_message",
		"id":         updated.ID,
		"channel_id": channelID,
		"content":    updated.Content,
		"edited_at":  updated.EditedAt.Time.Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, uuid.Nil, err
	}
	return data, channelID, nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) (uuid.UUID, error) {
	existing, err := s.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, ErrMessageNotFound
		}
		return uuid.Nil, err
	}

	if existing.UserID != userID && !isAdmin {
		return uuid.Nil, ErrForbidden
	}

	if err := s.queries.DeleteMessage(ctx, messageID); err != nil {
		return uuid.Nil, err
	}

	return existing.ChannelID.UUID, nil
}

func (s *MessageService) GetHistory(ctx context.Context, channelID uuid.UUID, beforeTime *time.Time, beforeID *uuid.UUID) ([]MessageInfo, bool, error) {
	nullChannelID := uuid.NullUUID{UUID: channelID, Valid: true}

	if beforeTime != nil && beforeID != nil {
		rows, err := s.queries.GetMessagesByChannel(ctx, db.GetMessagesByChannelParams{
			ChannelID:  nullChannelID,
			BeforeTime: *beforeTime,
			BeforeID:   *beforeID,
		})
		if err != nil {
			return nil, false, err
		}
		messages := make([]MessageInfo, len(rows))
		for i, row := range rows {
			messages[i] = messageInfoFromCursorRow(row)
		}
		return messages, len(rows) == 50, nil
	}

	rows, err := s.queries.GetLatestMessagesByChannel(ctx, nullChannelID)
	if err != nil {
		return nil, false, err
	}
	messages := make([]MessageInfo, len(rows))
	for i, row := range rows {
		messages[i] = messageInfoFromRow(row)
	}
	return messages, len(rows) == 50, nil
}
