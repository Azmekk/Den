package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/Azmekk/den/internal/db"
)

var (
	ErrMediaTooLarge   = errors.New("file too large")
	ErrMediaBadFormat  = errors.New("unsupported media format")
)

const (
	maxImageSize = 25 * 1024 * 1024  // 25MB
	maxVideoSize = 100 * 1024 * 1024 // 100MB
)

type MediaService struct {
	queries *db.Queries
	bucket  *BucketService
}

func NewMediaService(queries *db.Queries, bucket *BucketService) *MediaService {
	return &MediaService{queries: queries, bucket: bucket}
}

func (s *MediaService) IsConfigured() bool {
	return s.bucket != nil
}

func (s *MediaService) UploadImage(ctx context.Context, uploaderID uuid.UUID, fileData []byte) (string, error) {
	if s.bucket == nil {
		return "", ErrUploadsDisabled
	}
	if len(fileData) > maxImageSize {
		return "", ErrMediaTooLarge
	}

	ext, contentType, err := detectImageFormat(fileData)
	if err != nil {
		return "", err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(fileData))

	existing, err := s.queries.GetMediaUploadByHash(ctx, hash)
	if err == nil {
		_ = s.queries.ExtendMediaUploadExpiry(ctx, existing.ID)
		return s.bucket.PublicURL(existing.BucketKey), nil
	}

	key := "images/" + uuid.New().String() + ext
	if err := s.bucket.Upload(ctx, key, fileData, contentType); err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	_, err = s.queries.InsertMediaUpload(ctx, db.InsertMediaUploadParams{
		UploaderID:  uploaderID,
		BucketKey:   key,
		ContentHash: hash,
		MediaType:   "image",
	})
	if err != nil {
		_ = s.bucket.Delete(ctx, key)
		return "", err
	}

	return s.bucket.PublicURL(key), nil
}

func (s *MediaService) UploadVideo(ctx context.Context, uploaderID uuid.UUID, fileData []byte) (string, error) {
	if s.bucket == nil {
		return "", ErrUploadsDisabled
	}
	if len(fileData) > maxVideoSize {
		return "", ErrMediaTooLarge
	}

	ext, contentType, err := detectVideoFormat(fileData)
	if err != nil {
		return "", err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(fileData))

	existing, err := s.queries.GetMediaUploadByHash(ctx, hash)
	if err == nil {
		_ = s.queries.ExtendMediaUploadExpiry(ctx, existing.ID)
		return s.bucket.PublicURL(existing.BucketKey), nil
	}

	key := "videos/" + uuid.New().String() + ext
	if err := s.bucket.Upload(ctx, key, fileData, contentType); err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	_, err = s.queries.InsertMediaUpload(ctx, db.InsertMediaUploadParams{
		UploaderID:  uploaderID,
		BucketKey:   key,
		ContentHash: hash,
		MediaType:   "video",
	})
	if err != nil {
		_ = s.bucket.Delete(ctx, key)
		return "", err
	}

	return s.bucket.PublicURL(key), nil
}

func (s *MediaService) CleanupExpired(ctx context.Context) error {
	rows, err := s.queries.GetExpiredMediaUploads(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	for _, row := range rows {
		_ = s.bucket.Delete(ctx, row.BucketKey)
	}

	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return s.queries.DeleteMediaUploadsByIDs(ctx, ids)
}

func (s *MediaService) RunCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.CleanupExpired(ctx); err != nil {
				log.Printf("media cleanup error: %v", err)
			}
		}
	}
}

func detectImageFormat(data []byte) (ext string, contentType string, err error) {
	switch {
	case len(data) >= 4 && string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WEBP":
		return ".webp", "image/webp", nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return ".png", "image/png", nil
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return ".jpg", "image/jpeg", nil
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return ".gif", "image/gif", nil
	default:
		return "", "", ErrMediaBadFormat
	}
}

func detectVideoFormat(data []byte) (ext string, contentType string, err error) {
	switch {
	case len(data) >= 8 && string(data[4:8]) == "ftyp":
		return ".mp4", "video/mp4", nil
	case len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3:
		return ".webm", "video/webm", nil
	default:
		return "", "", ErrMediaBadFormat
	}
}

// UpdateAvatar handles avatar upload for a user.
func (s *MediaService) UpdateAvatar(ctx context.Context, userID uuid.UUID, fileData []byte, queries *db.Queries) (PublicUserInfo, error) {
	if s.bucket == nil {
		return PublicUserInfo{}, ErrUploadsDisabled
	}
	if len(fileData) > 5*1024*1024 {
		return PublicUserInfo{}, ErrMediaTooLarge
	}

	ext, contentType, err := detectImageFormat(fileData)
	if err != nil {
		return PublicUserInfo{}, ErrMediaBadFormat
	}
	// Only allow image formats for avatars (no video)
	_ = ext

	key := "avatars/" + userID.String() + ext
	if err := s.bucket.Upload(ctx, key, fileData, contentType); err != nil {
		return PublicUserInfo{}, fmt.Errorf("upload failed: %w", err)
	}

	avatarURL := s.bucket.PublicURL(key)
	user, err := queries.UpdateUserAvatarUrl(ctx, db.UpdateUserAvatarUrlParams{
		ID:        userID,
		AvatarUrl: sql.NullString{String: avatarURL, Valid: true},
	})
	if err != nil {
		return PublicUserInfo{}, err
	}

	return publicUserInfoFromDB(user), nil
}
