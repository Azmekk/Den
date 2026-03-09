package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	_ "image/png"
	"regexp"
	"strings"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/martinmckenna/den/internal/db"
)

var (
	ErrEmoteNotFound    = errors.New("emote not found")
	ErrEmoteNameInvalid = errors.New("emote name must be 2-32 alphanumeric/underscore characters")
	ErrEmoteNameTaken   = errors.New("emote name already taken")
	ErrEmoteTooLarge    = errors.New("emote file must be 256KB or less")
	ErrEmoteBadFormat   = errors.New("emote must be PNG, GIF, or WebP")
	ErrEmoteDimensions  = errors.New("emote must be 128x128 pixels or smaller")
	ErrUploadsDisabled  = errors.New("uploads not configured")

	emoteNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{2,32}$`)
	emotePattern     = regexp.MustCompile(`:([a-zA-Z0-9_]{2,32}):`)
)

const maxEmoteSize = 256 * 1024 // 256KB

type EmoteService struct {
	queries *db.Queries
	bucket  *BucketService
}

func NewEmoteService(queries *db.Queries, bucket *BucketService) *EmoteService {
	return &EmoteService{queries: queries, bucket: bucket}
}

func (s *EmoteService) IsConfigured() bool {
	return s.bucket != nil
}

type EmoteInfo struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	URL  string    `json:"url"`
}

func (s *EmoteService) Create(ctx context.Context, name string, uploaderID uuid.UUID, fileData []byte, contentType string) (*EmoteInfo, error) {
	if s.bucket == nil {
		return nil, ErrUploadsDisabled
	}

	if !emoteNamePattern.MatchString(name) {
		return nil, ErrEmoteNameInvalid
	}

	if len(fileData) > maxEmoteSize {
		return nil, ErrEmoteTooLarge
	}

	// Detect format from magic bytes and decode to check dimensions
	format, isAnimatedGif, err := detectAndValidateImage(fileData)
	if err != nil {
		return nil, err
	}

	// Determine storage format and content type
	var ext, storeCT string
	if isAnimatedGif {
		ext = ".gif"
		storeCT = "image/gif"
	} else {
		// Keep original format (no CGO-dependent WebP encoding)
		switch format {
		case "png":
			ext = ".png"
			storeCT = "image/png"
		case "gif":
			ext = ".gif"
			storeCT = "image/gif"
		case "webp":
			ext = ".webp"
			storeCT = "image/webp"
		default:
			return nil, ErrEmoteBadFormat
		}
	}

	emoteID := uuid.New()
	filename := emoteID.String() + ext
	key := "emotes/" + filename

	if err := s.bucket.Upload(ctx, key, fileData, storeCT); err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	emote, err := s.queries.CreateEmote(ctx, db.CreateEmoteParams{
		Name:       name,
		Filename:   filename,
		UploadedBy: uploaderID,
	})
	if err != nil {
		// Clean up uploaded file on DB error
		_ = s.bucket.Delete(ctx, key)
		if isUniqueViolation(err) {
			return nil, ErrEmoteNameTaken
		}
		return nil, err
	}

	return &EmoteInfo{
		ID:   emote.ID,
		Name: emote.Name,
		URL:  s.bucket.PublicURL(key),
	}, nil
}

func (s *EmoteService) Delete(ctx context.Context, emoteID uuid.UUID) error {
	filename, err := s.queries.DeleteEmote(ctx, emoteID)
	if err != nil {
		return ErrEmoteNotFound
	}

	if s.bucket != nil {
		_ = s.bucket.Delete(ctx, "emotes/"+filename)
	}
	return nil
}

func (s *EmoteService) List(ctx context.Context) ([]EmoteInfo, error) {
	emotes, err := s.queries.ListEmotes(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]EmoteInfo, len(emotes))
	for i, e := range emotes {
		url := "/api/emotes/" + e.ID.String() + "/image"
		result[i] = EmoteInfo{
			ID:   e.ID,
			Name: e.Name,
			URL:  url,
		}
	}
	return result, nil
}

func (s *EmoteService) GetImageURL(ctx context.Context, emoteID uuid.UUID) (string, error) {
	emote, err := s.queries.GetEmoteByID(ctx, emoteID)
	if err != nil {
		return "", ErrEmoteNotFound
	}
	if s.bucket == nil {
		return "", ErrUploadsDisabled
	}
	return s.bucket.PublicURL("emotes/" + emote.Filename), nil
}

// ResolveTokens escapes angle brackets and replaces :shortcode: with <emote:uuid> tokens.
func (s *EmoteService) ResolveTokens(ctx context.Context, content string) (string, error) {
	// Escape angle brackets to prevent users from faking emote tokens
	content = strings.ReplaceAll(content, "<", "&lt;")
	content = strings.ReplaceAll(content, ">", "&gt;")

	// Find all shortcodes
	matches := emotePattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	// Collect unique names
	nameSet := make(map[string]bool)
	for _, m := range matches {
		name := content[m[2]:m[3]]
		nameSet[name] = true
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}

	// Batch query
	emotes, err := s.queries.GetEmotesByNames(ctx, names)
	if err != nil {
		return content, nil // On error, return escaped content without replacements
	}

	nameToID := make(map[string]uuid.UUID, len(emotes))
	for _, e := range emotes {
		nameToID[e.Name] = e.ID
	}

	// Replace from end to start to preserve indices
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		name := content[m[2]:m[3]]
		if id, ok := nameToID[name]; ok {
			token := "<emote:" + id.String() + ">"
			content = content[:m[0]] + token + content[m[1]:]
		}
	}

	return content, nil
}

// EscapeContent escapes angle brackets without resolving emote tokens.
// Used when EmoteService is nil.
func EscapeContent(content string) string {
	content = strings.ReplaceAll(content, "<", "&lt;")
	content = strings.ReplaceAll(content, ">", "&gt;")
	return content
}

func detectAndValidateImage(data []byte) (format string, isAnimatedGif bool, err error) {
	// Check magic bytes
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		format = "png"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		format = "gif"
	case len(data) >= 4 && string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WEBP":
		format = "webp"
	default:
		return "", false, ErrEmoteBadFormat
	}

	// Check for animated GIF
	if format == "gif" {
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return "", false, ErrEmoteBadFormat
		}
		isAnimatedGif = len(g.Image) > 1
		// Check dimensions from first frame
		if len(g.Image) > 0 {
			bounds := g.Image[0].Bounds()
			w := bounds.Dx()
			h := bounds.Dy()
			if w > 128 || h > 128 {
				return "", false, ErrEmoteDimensions
			}
		}
		return format, isAnimatedGif, nil
	}

	// Decode to check dimensions
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", false, ErrEmoteBadFormat
	}
	bounds := img.Bounds()
	if bounds.Dx() > 128 || bounds.Dy() > 128 {
		return "", false, ErrEmoteDimensions
	}

	return format, false, nil
}
