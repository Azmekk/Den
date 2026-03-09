package service

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type BucketService struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

// NewBucketService creates a BucketService from environment variables.
// Returns nil if required env vars are missing.
func NewBucketService() *BucketService {
	endpoint := os.Getenv("BUCKET_ENDPOINT")
	bucket := os.Getenv("BUCKET_NAME")
	region := os.Getenv("BUCKET_REGION")
	accessKey := os.Getenv("BUCKET_ACCESS_KEY")
	secretKey := os.Getenv("BUCKET_SECRET_KEY")
	publicURL := os.Getenv("BUCKET_PUBLIC_URL")

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil
	}
	if region == "" {
		region = "auto"
	}

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	})

	return &BucketService{
		client:    client,
		bucket:    bucket,
		publicURL: publicURL,
	}
}

func (b *BucketService) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (b *BucketService) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (b *BucketService) PublicURL(key string) string {
	if b.publicURL != "" {
		return fmt.Sprintf("%s/%s", b.publicURL, key)
	}
	return fmt.Sprintf("/%s", key)
}
