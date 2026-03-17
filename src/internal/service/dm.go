package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrDMPairNotFound = errors.New("dm pair not found")
	ErrNotInDMPair    = errors.New("user not in dm pair")
	ErrCannotDMSelf   = errors.New("cannot dm yourself")
)

type DMService struct {
	queries     *db.Queries
	emoteSvc    *EmoteService
	getMaxChars func() int
}

func NewDMService(queries *db.Queries, emoteSvc *EmoteService, getMaxChars func() int) *DMService {
	return &DMService{queries: queries, emoteSvc: emoteSvc, getMaxChars: getMaxChars}
}

type DMPairInfo struct {
	ID               uuid.UUID `json:"id"`
	OtherUserID      uuid.UUID `json:"other_user_id"`
	OtherUsername    string    `json:"other_username"`
	OtherDisplayName string    `json:"other_display_name,omitempty"`
	OtherAvatarURL   string    `json:"other_avatar_url,omitempty"`
	CreatedAt        string    `json:"created_at"`
}

type DMMessageInfo struct {
	ID              uuid.UUID  `json:"id"`
	DMPairID        uuid.UUID  `json:"dm_pair_id"`
	UserID          uuid.UUID  `json:"user_id"`
	Username        string     `json:"username"`
	DisplayName     string     `json:"display_name,omitempty"`
	AvatarURL       string     `json:"avatar_url,omitempty"`
	Content         string     `json:"content"`
	Pinned          bool       `json:"pinned"`
	EditedAt        string     `json:"edited_at,omitempty"`
	CreatedAt       string     `json:"created_at"`
	IsReply         bool       `json:"is_reply,omitempty"`
	ReplyToID       *uuid.UUID `json:"reply_to_id,omitempty"`
	ReplyToContent  string     `json:"reply_to_content,omitempty"`
	ReplyToUserID   *uuid.UUID `json:"reply_to_user_id,omitempty"`
	ReplyToUsername  string     `json:"reply_to_username,omitempty"`
}

func (s *DMService) CreateOrGetDMPair(ctx context.Context, currentUserID, otherUserID uuid.UUID) (*DMPairInfo, error) {
	if currentUserID == otherUserID {
		return nil, ErrCannotDMSelf
	}

	pair, err := s.queries.CreateDMPair(ctx, db.CreateDMPairParams{
		UserA: currentUserID,
		UserB: otherUserID,
	})
	if err != nil {
		return nil, err
	}

	// Determine other user
	otherID := pair.UserB
	if otherID == currentUserID {
		otherID = pair.UserA
	}

	// Look up other user's info
	user, err := s.queries.GetUserByID(ctx, otherID)
	if err != nil {
		return nil, err
	}

	info := &DMPairInfo{
		ID:            pair.ID,
		OtherUserID:   otherID,
		OtherUsername: user.Username,
		CreatedAt:     pair.CreatedAt.Format(time.RFC3339Nano),
	}
	if user.DisplayName.Valid {
		info.OtherDisplayName = user.DisplayName.String
	}
	if user.AvatarUrl.Valid {
		info.OtherAvatarURL = user.AvatarUrl.String
	}
	return info, nil
}

func (s *DMService) ListConversations(ctx context.Context, userID uuid.UUID) ([]DMPairInfo, error) {
	rows, err := s.queries.ListDMPairsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]DMPairInfo, len(rows))
	for i, r := range rows {
		info := DMPairInfo{
			ID:            r.ID,
			OtherUserID:   r.OtherUserID,
			OtherUsername: r.OtherUsername,
			CreatedAt:     r.CreatedAt.Format(time.RFC3339Nano),
		}
		if r.OtherDisplayName.Valid {
			info.OtherDisplayName = r.OtherDisplayName.String
		}
		if r.OtherAvatarUrl.Valid {
			info.OtherAvatarURL = r.OtherAvatarUrl.String
		}
		result[i] = info
	}
	return result, nil
}

func (s *DMService) GetDMHistory(ctx context.Context, dmPairID uuid.UUID, beforeTime *time.Time, beforeID *uuid.UUID) ([]DMMessageInfo, bool, error) {
	nullDMPairID := uuid.NullUUID{UUID: dmPairID, Valid: true}

	if beforeTime != nil && beforeID != nil {
		rows, err := s.queries.GetDMMessagesByPair(ctx, db.GetDMMessagesByPairParams{
			DmPairID:   nullDMPairID,
			BeforeTime: *beforeTime,
			BeforeID:   *beforeID,
		})
		if err != nil {
			return nil, false, err
		}
		messages := make([]DMMessageInfo, len(rows))
		for i, row := range rows {
			messages[i] = dmMessageInfoFromCursorRow(row)
		}
		return messages, len(rows) == 50, nil
	}

	rows, err := s.queries.GetLatestDMMessages(ctx, nullDMPairID)
	if err != nil {
		return nil, false, err
	}
	messages := make([]DMMessageInfo, len(rows))
	for i, row := range rows {
		messages[i] = dmMessageInfoFromLatestRow(row)
	}
	return messages, len(rows) == 50, nil
}

func (s *DMService) SendDMMessage(ctx context.Context, dmPairID, userID uuid.UUID, username, content string, replyToID *uuid.UUID) ([]byte, []uuid.UUID, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > s.getMaxChars() {
		return nil, nil, ErrInvalidInput
	}

	if s.emoteSvc != nil {
		content, _ = s.emoteSvc.ResolveTokens(ctx, content)
	} else {
		content = EscapeContent(content)
	}

	content, mentionedIDs, _ := s.resolveMentions(ctx, content)

	// Filter mentions to only DM participants
	pair, err := s.queries.GetDMPair(ctx, dmPairID)
	if err == nil {
		filtered := mentionedIDs[:0]
		for _, uid := range mentionedIDs {
			if uid == pair.UserA || uid == pair.UserB {
				filtered = append(filtered, uid)
			}
		}
		mentionedIDs = filtered
	}

	createParams := db.CreateDMMessageParams{
		DmPairID: uuid.NullUUID{UUID: dmPairID, Valid: true},
		UserID:   userID,
		Content:  content,
	}
	if replyToID != nil {
		createParams.ReplyToID = uuid.NullUUID{UUID: *replyToID, Valid: true}
		createParams.IsReply = true
	}

	msg, err := s.queries.CreateDMMessage(ctx, createParams)
	if err != nil {
		return nil, nil, err
	}

	// Auto-ping the replied-to user (if they are a DM participant and not the sender)
	var repliedMsg *db.GetMessageByIDRow
	if replyToID != nil {
		row, lookupErr := s.queries.GetMessageByID(ctx, *replyToID)
		if lookupErr == nil {
			repliedMsg = &row
			if row.UserID != userID && err == nil && (row.UserID == pair.UserA || row.UserID == pair.UserB) {
				alreadyMentioned := false
				for _, uid := range mentionedIDs {
					if uid == row.UserID {
						alreadyMentioned = true
						break
					}
				}
				if !alreadyMentioned {
					mentionedIDs = append(mentionedIDs, row.UserID)
				}
			}
		}
	}

	for _, uid := range mentionedIDs {
		_ = s.queries.InsertMention(ctx, db.InsertMentionParams{
			MessageID: msg.ID,
			UserID:    uid,
		})
	}

	mentionedStrings := make([]string, len(mentionedIDs))
	for index, uid := range mentionedIDs {
		mentionedStrings[index] = uid.String()
	}

	envelope := map[string]any{
		"type":               "new_dm",
		"id":                 msg.ID,
		"dm_pair_id":         dmPairID,
		"user_id":            userID,
		"username":           username,
		"content":            msg.Content,
		"pinned":             false,
		"created_at":         msg.CreatedAt.Format(time.RFC3339Nano),
		"mentioned_user_ids": mentionedStrings,
	}

	// Include reply info in the envelope
	if replyToID != nil && repliedMsg != nil {
		replyContent := repliedMsg.Content
		if len(replyContent) > 200 {
			replyContent = replyContent[:200] + "..."
		}
		envelope["is_reply"] = true
		envelope["reply_to_id"] = replyToID.String()
		envelope["reply_to_content"] = replyContent
		envelope["reply_to_user_id"] = repliedMsg.UserID.String()
		repliedUser, userErr := s.queries.GetUserByID(ctx, repliedMsg.UserID)
		if userErr == nil {
			envelope["reply_to_username"] = repliedUser.Username
		}
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, nil, err
	}
	return data, mentionedIDs, nil
}

func (s *DMService) ValidateUserInPair(ctx context.Context, dmPairID, userID uuid.UUID) (uuid.UUID, error) {
	pair, err := s.queries.GetDMPair(ctx, dmPairID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, ErrDMPairNotFound
		}
		return uuid.Nil, err
	}

	if pair.UserA == userID {
		return pair.UserB, nil
	}
	if pair.UserB == userID {
		return pair.UserA, nil
	}
	return uuid.Nil, ErrNotInDMPair
}

func (s *DMService) GetPinnedDMMessages(ctx context.Context, dmPairID uuid.UUID) ([]DMMessageInfo, error) {
	rows, err := s.queries.GetPinnedDMMessages(ctx, uuid.NullUUID{UUID: dmPairID, Valid: true})
	if err != nil {
		return nil, err
	}
	messages := make([]DMMessageInfo, len(rows))
	for index, row := range rows {
		messages[index] = DMMessageInfo{
			ID:        row.ID,
			DMPairID:  row.DmPairID.UUID,
			UserID:    row.UserID,
			Username:  row.Username,
			Content:   row.Content,
			Pinned:    row.Pinned,
			CreatedAt: row.CreatedAt.Format(time.RFC3339Nano),
		}
		if row.DisplayName.Valid {
			messages[index].DisplayName = row.DisplayName.String
		}
		if row.AvatarUrl.Valid {
			messages[index].AvatarURL = row.AvatarUrl.String
		}
		if row.EditedAt.Valid {
			messages[index].EditedAt = row.EditedAt.Time.Format(time.RFC3339Nano)
		}
		populateDMReplyFields(&messages[index], row.ReplyToID, row.IsReply, row.ReplyToContent, row.ReplyToUserID, row.ReplyToUsername)
	}
	return messages, nil
}

func (s *DMService) resolveMentions(ctx context.Context, content string) (string, []uuid.UUID, error) {
	matches := mentionPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil, nil
	}

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

func populateDMReplyFields(info *DMMessageInfo, replyToID uuid.NullUUID, isReply bool, replyToContent sql.NullString, replyToUserID uuid.NullUUID, replyToUsername sql.NullString) {
	info.IsReply = isReply
	if replyToID.Valid {
		info.ReplyToID = &replyToID.UUID
	}
	if replyToContent.Valid {
		content := replyToContent.String
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		info.ReplyToContent = content
	}
	if replyToUserID.Valid {
		info.ReplyToUserID = &replyToUserID.UUID
	}
	if replyToUsername.Valid {
		info.ReplyToUsername = replyToUsername.String
	}
}

func dmMessageInfoFromLatestRow(row db.GetLatestDMMessagesRow) DMMessageInfo {
	info := DMMessageInfo{
		ID:        row.ID,
		DMPairID:  row.DmPairID.UUID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
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
	populateDMReplyFields(&info, row.ReplyToID, row.IsReply, row.ReplyToContent, row.ReplyToUserID, row.ReplyToUsername)
	return info
}

func dmMessageInfoFromCursorRow(row db.GetDMMessagesByPairRow) DMMessageInfo {
	info := DMMessageInfo{
		ID:        row.ID,
		DMPairID:  row.DmPairID.UUID,
		UserID:    row.UserID,
		Username:  row.Username,
		Content:   row.Content,
		Pinned:    row.Pinned,
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
	populateDMReplyFields(&info, row.ReplyToID, row.IsReply, row.ReplyToContent, row.ReplyToUserID, row.ReplyToUsername)
	return info
}
