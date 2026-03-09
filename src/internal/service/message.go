package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var mentionPattern = regexp.MustCompile(`@([a-zA-Z0-9_]+)`)

var (
	ErrMessageNotFound = errors.New("message not found")
	ErrForbidden       = errors.New("forbidden")
)

type MessageService struct {
	queries  *db.Queries
	emoteSvc *EmoteService
}

func NewMessageService(queries *db.Queries, emoteSvc *EmoteService) *MessageService {
	return &MessageService{queries: queries, emoteSvc: emoteSvc}
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

func (s *MessageService) SendMessage(ctx context.Context, channelID, userID uuid.UUID, username, content string) ([]byte, []uuid.UUID, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 2000 {
		return nil, nil, ErrInvalidInput
	}

	if s.emoteSvc != nil {
		content, _ = s.emoteSvc.ResolveTokens(ctx, content)
	} else {
		content = EscapeContent(content)
	}

	content, mentionedIDs, _ := s.resolveMentions(ctx, content)

	msg, err := s.queries.CreateMessage(ctx, db.CreateMessageParams{
		ChannelID: uuid.NullUUID{UUID: channelID, Valid: true},
		UserID:    userID,
		Content:   content,
	})
	if err != nil {
		return nil, nil, err
	}

	// Insert mention rows
	for _, uid := range mentionedIDs {
		_ = s.queries.InsertMention(ctx, db.InsertMentionParams{
			MessageID: msg.ID,
			UserID:    uid,
		})
	}

	mentionedStrings := make([]string, len(mentionedIDs))
	for i, uid := range mentionedIDs {
		mentionedStrings[i] = uid.String()
	}

	envelope := map[string]any{
		"type":               "new_message",
		"id":                 msg.ID,
		"channel_id":         channelID,
		"user_id":            userID,
		"username":           username,
		"content":            msg.Content,
		"created_at":         msg.CreatedAt.Format(time.RFC3339Nano),
		"mentioned_user_ids": mentionedStrings,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, nil, err
	}
	return data, mentionedIDs, nil
}

func (s *MessageService) EditMessage(ctx context.Context, messageID, userID uuid.UUID, content string) ([]byte, uuid.UUID, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 2000 {
		return nil, uuid.Nil, ErrInvalidInput
	}

	if s.emoteSvc != nil {
		content, _ = s.emoteSvc.ResolveTokens(ctx, content)
	} else {
		content = EscapeContent(content)
	}

	content, mentionedIDs, _ := s.resolveMentions(ctx, content)

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

	// Re-resolve mentions: delete old, insert new
	_ = s.queries.DeleteMentionsByMessage(ctx, messageID)
	for _, uid := range mentionedIDs {
		_ = s.queries.InsertMention(ctx, db.InsertMentionParams{
			MessageID: messageID,
			UserID:    uid,
		})
	}

	mentionedStrings := make([]string, len(mentionedIDs))
	for i, uid := range mentionedIDs {
		mentionedStrings[i] = uid.String()
	}

	channelID := existing.ChannelID.UUID
	envelope := map[string]any{
		"type":               "edit_message",
		"id":                 updated.ID,
		"channel_id":         channelID,
		"content":            updated.Content,
		"edited_at":          updated.EditedAt.Time.Format(time.RFC3339Nano),
		"mentioned_user_ids": mentionedStrings,
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

// resolveMentions finds @username patterns and replaces them with <mention:uuid> tokens.
func (s *MessageService) resolveMentions(ctx context.Context, content string) (string, []uuid.UUID, error) {
	matches := mentionPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil, nil
	}

	// Collect unique usernames
	nameSet := make(map[string]bool)
	for _, m := range matches {
		name := content[m[2]:m[3]]
		nameSet[name] = true
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}

	users, err := s.queries.GetUsersByUsernames(ctx, names)
	if err != nil {
		return content, nil, err
	}

	nameToID := make(map[string]uuid.UUID, len(users))
	for _, u := range users {
		nameToID[u.Username] = u.ID
	}

	// Replace from end to start to preserve indices
	var mentionedIDs []uuid.UUID
	seen := make(map[uuid.UUID]bool)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		name := content[m[2]:m[3]]
		if id, ok := nameToID[name]; ok {
			token := "<mention:" + id.String() + ">"
			content = content[:m[0]] + token + content[m[1]:]
			if !seen[id] {
				seen[id] = true
				mentionedIDs = append(mentionedIDs, id)
			}
		}
	}

	return content, mentionedIDs, nil
}
