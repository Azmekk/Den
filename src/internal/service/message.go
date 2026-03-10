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
	ID          uuid.UUID  `json:"id"`
	ChannelID   *uuid.UUID `json:"channel_id,omitempty"`
	DMPairID    *uuid.UUID `json:"dm_pair_id,omitempty"`
	UserID      uuid.UUID  `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	Content     string     `json:"content"`
	Pinned      bool       `json:"pinned"`
	EditedAt    string     `json:"edited_at,omitempty"`
	CreatedAt   string     `json:"created_at"`
}

func messageInfoFromRow(row db.GetLatestMessagesByChannelRow) MessageInfo {
	info := MessageInfo{
		ID:        row.ID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.ChannelID.Valid {
		info.ChannelID = &row.ChannelID.UUID
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
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.ChannelID.Valid {
		info.ChannelID = &row.ChannelID.UUID
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

func messageInfoFromPinnedChannelRow(row db.GetPinnedMessagesByChannelRow) MessageInfo {
	info := MessageInfo{
		ID:        row.ID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.ChannelID.Valid {
		info.ChannelID = &row.ChannelID.UUID
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

	content, mentionedIDs, mentionedEveryone := s.resolveMentions(ctx, content)

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
		"pinned":             false,
		"created_at":         msg.CreatedAt.Format(time.RFC3339Nano),
		"mentioned_user_ids": mentionedStrings,
	}
	if mentionedEveryone {
		envelope["mentioned_everyone"] = true
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, nil, err
	}
	return data, mentionedIDs, nil
}

func (s *MessageService) EditMessage(ctx context.Context, messageID, userID uuid.UUID, content string) ([]byte, uuid.UUID, uuid.UUID, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 2000 {
		return nil, uuid.Nil, uuid.Nil, ErrInvalidInput
	}

	if s.emoteSvc != nil {
		content, _ = s.emoteSvc.ResolveTokens(ctx, content)
	} else {
		content = EscapeContent(content)
	}

	content, mentionedIDs, mentionedEveryone := s.resolveMentions(ctx, content)

	existing, err := s.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uuid.Nil, uuid.Nil, ErrMessageNotFound
		}
		return nil, uuid.Nil, uuid.Nil, err
	}

	if existing.UserID != userID {
		return nil, uuid.Nil, uuid.Nil, ErrForbidden
	}

	updated, err := s.queries.UpdateMessageContent(ctx, db.UpdateMessageContentParams{
		ID:      messageID,
		Content: content,
	})
	if err != nil {
		return nil, uuid.Nil, uuid.Nil, err
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
	dmPairID := existing.DmPairID.UUID
	envelope := map[string]any{
		"type":               "edit_message",
		"id":                 updated.ID,
		"content":            updated.Content,
		"edited_at":          updated.EditedAt.Time.Format(time.RFC3339Nano),
		"mentioned_user_ids": mentionedStrings,
	}
	if mentionedEveryone {
		envelope["mentioned_everyone"] = true
	}
	if existing.ChannelID.Valid {
		envelope["channel_id"] = channelID
	}
	if existing.DmPairID.Valid {
		envelope["dm_pair_id"] = dmPairID
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, uuid.Nil, uuid.Nil, err
	}
	return data, channelID, dmPairID, nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) (uuid.UUID, uuid.UUID, error) {
	existing, err := s.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, uuid.Nil, ErrMessageNotFound
		}
		return uuid.Nil, uuid.Nil, err
	}

	if existing.UserID != userID && !isAdmin {
		return uuid.Nil, uuid.Nil, ErrForbidden
	}

	if err := s.queries.DeleteMessage(ctx, messageID); err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return existing.ChannelID.UUID, existing.DmPairID.UUID, nil
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

func (s *MessageService) PinMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) ([]byte, error) {
	existing, err := s.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}

	// DM messages: only participants can pin (admin bypass not allowed)
	if existing.DmPairID.Valid {
		pair, err := s.queries.GetDMPair(ctx, existing.DmPairID.UUID)
		if err != nil {
			return nil, err
		}
		if userID != pair.UserA && userID != pair.UserB {
			return nil, ErrForbidden
		}
	} else if existing.UserID != userID && !isAdmin {
		return nil, ErrForbidden
	}

	msg, err := s.queries.SetMessagePinned(ctx, db.SetMessagePinnedParams{
		ID:     messageID,
		Pinned: true,
	})
	if err != nil {
		return nil, err
	}

	envelope := map[string]any{
		"type":   "pin_message",
		"id":     msg.ID,
		"pinned": true,
	}
	if msg.ChannelID.Valid {
		envelope["channel_id"] = msg.ChannelID.UUID
	}
	if msg.DmPairID.Valid {
		envelope["dm_pair_id"] = msg.DmPairID.UUID
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *MessageService) UnpinMessage(ctx context.Context, messageID, userID uuid.UUID, isAdmin bool) ([]byte, error) {
	existing, err := s.queries.GetMessageByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}

	// DM messages: only participants can unpin (admin bypass not allowed)
	if existing.DmPairID.Valid {
		pair, err := s.queries.GetDMPair(ctx, existing.DmPairID.UUID)
		if err != nil {
			return nil, err
		}
		if userID != pair.UserA && userID != pair.UserB {
			return nil, ErrForbidden
		}
	} else if existing.UserID != userID && !isAdmin {
		return nil, ErrForbidden
	}

	msg, err := s.queries.SetMessagePinned(ctx, db.SetMessagePinnedParams{
		ID:     messageID,
		Pinned: false,
	})
	if err != nil {
		return nil, err
	}

	envelope := map[string]any{
		"type":   "unpin_message",
		"id":     msg.ID,
		"pinned": false,
	}
	if msg.ChannelID.Valid {
		envelope["channel_id"] = msg.ChannelID.UUID
	}
	if msg.DmPairID.Valid {
		envelope["dm_pair_id"] = msg.DmPairID.UUID
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *MessageService) GetPinnedMessages(ctx context.Context, channelID uuid.UUID) ([]MessageInfo, error) {
	rows, err := s.queries.GetPinnedMessagesByChannel(ctx, uuid.NullUUID{UUID: channelID, Valid: true})
	if err != nil {
		return nil, err
	}
	messages := make([]MessageInfo, len(rows))
	for i, row := range rows {
		messages[i] = messageInfoFromPinnedChannelRow(row)
	}
	return messages, nil
}

type SearchResult struct {
	ID          uuid.UUID `json:"id"`
	ChannelID   uuid.UUID `json:"channel_id"`
	ChannelName string    `json:"channel_name"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Content     string    `json:"content"`
	Pinned      bool      `json:"pinned"`
	EditedAt    string    `json:"edited_at,omitempty"`
	CreatedAt   string    `json:"created_at"`
}

func (s *MessageService) SearchMessages(ctx context.Context, query *string, channelID, authorID *uuid.UUID, afterTime, beforeTime *time.Time) ([]SearchResult, error) {
	params := db.SearchMessagesParams{}
	if query != nil {
		params.Query = sql.NullString{String: *query, Valid: true}
	}
	if channelID != nil {
		params.ChannelID = uuid.NullUUID{UUID: *channelID, Valid: true}
	}
	if authorID != nil {
		params.AuthorID = uuid.NullUUID{UUID: *authorID, Valid: true}
	}
	if afterTime != nil {
		params.AfterTime = sql.NullTime{Time: *afterTime, Valid: true}
	}
	if beforeTime != nil {
		params.BeforeTime = sql.NullTime{Time: *beforeTime, Valid: true}
	}

	rows, err := s.queries.SearchMessages(ctx, params)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(rows))
	for i, row := range rows {
		r := SearchResult{
			ID:          row.ID,
			ChannelID:   row.ChannelID.UUID,
			ChannelName: row.ChannelName,
			UserID:      row.UserID,
			Username:    row.Username,
			Content:     row.Content,
			Pinned:      row.Pinned,
			CreatedAt:   row.CreatedAt.Format(time.RFC3339Nano),
		}
		if row.DisplayName.Valid {
			r.DisplayName = row.DisplayName.String
		}
		if row.AvatarUrl.Valid {
			r.AvatarURL = row.AvatarUrl.String
		}
		if row.EditedAt.Valid {
			r.EditedAt = row.EditedAt.Time.Format(time.RFC3339Nano)
		}
		results[i] = r
	}
	return results, nil
}

func messageInfoFromAroundRow(row db.GetMessagesAroundTargetRow) MessageInfo {
	info := MessageInfo{
		ID:        row.ID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.ChannelID.Valid {
		info.ChannelID = &row.ChannelID.UUID
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

func messageInfoFromAfterCursorRow(row db.GetMessagesAfterCursorRow) MessageInfo {
	info := MessageInfo{
		ID:        row.ID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
		CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
	}
	if row.ChannelID.Valid {
		info.ChannelID = &row.ChannelID.UUID
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

func (s *MessageService) GetMessagesAround(ctx context.Context, channelID, targetMessageID uuid.UUID) ([]MessageInfo, bool, bool, error) {
	rows, err := s.queries.GetMessagesAroundTarget(ctx, db.GetMessagesAroundTargetParams{
		ChannelID: uuid.NullUUID{UUID: channelID, Valid: true},
		TargetID:  targetMessageID,
	})
	if err != nil {
		return nil, false, false, err
	}

	messages := make([]MessageInfo, len(rows))
	targetFound := false
	beforeCount := 0
	afterCount := 0
	for i, row := range rows {
		messages[i] = messageInfoFromAroundRow(row)
		if row.ID == targetMessageID {
			targetFound = true
		} else if !targetFound {
			beforeCount++
		} else {
			afterCount++
		}
	}

	return messages, beforeCount == 25, afterCount == 25, nil
}

func (s *MessageService) GetNewer(ctx context.Context, channelID uuid.UUID, afterTime time.Time, afterID uuid.UUID) ([]MessageInfo, bool, error) {
	rows, err := s.queries.GetMessagesAfterCursor(ctx, db.GetMessagesAfterCursorParams{
		ChannelID: uuid.NullUUID{UUID: channelID, Valid: true},
		AfterTime: afterTime,
		AfterID:   afterID,
	})
	if err != nil {
		return nil, false, err
	}

	messages := make([]MessageInfo, len(rows))
	for i, row := range rows {
		messages[i] = messageInfoFromAfterCursorRow(row)
	}
	return messages, len(rows) == 50, nil
}

// resolveMentions finds @username patterns and replaces them with <mention:uuid> tokens.
// Returns the resolved content, mentioned user IDs, and whether @everyone was mentioned.
func (s *MessageService) resolveMentions(ctx context.Context, content string) (string, []uuid.UUID, bool) {
	matches := mentionPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil, false
	}

	// Collect unique usernames (excluding "everyone")
	nameSet := make(map[string]bool)
	for _, m := range matches {
		name := content[m[2]:m[3]]
		if strings.ToLower(name) != "everyone" {
			nameSet[name] = true
		}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}

	nameToID := make(map[string]uuid.UUID)
	if len(names) > 0 {
		users, err := s.queries.GetUsersByUsernames(ctx, names)
		if err == nil {
			for _, u := range users {
				nameToID[u.Username] = u.ID
			}
		}
	}

	// Replace from end to start to preserve indices
	var mentionedIDs []uuid.UUID
	mentionedEveryone := false
	seen := make(map[uuid.UUID]bool)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		name := content[m[2]:m[3]]
		if strings.ToLower(name) == "everyone" {
			content = content[:m[0]] + "<mention:everyone>" + content[m[1]:]
			mentionedEveryone = true
		} else if id, ok := nameToID[name]; ok {
			token := "<mention:" + id.String() + ">"
			content = content[:m[0]] + token + content[m[1]:]
			if !seen[id] {
				seen[id] = true
				mentionedIDs = append(mentionedIDs, id)
			}
		}
	}

	return content, mentionedIDs, mentionedEveryone
}
