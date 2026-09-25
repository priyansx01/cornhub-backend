// Package storage provides MinIO object storage connectivity.
package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/priyansx01/corn-hub-clone/internal/config"
)

// Client wraps the MinIO SDK with LMS-specific operations.
type Client struct {
	mc          *minio.Client
	rawBucket   string
	hlsBucket   string
	thumbBucket string
	publicURL   string
}

// NewClient creates a MinIO client and ensures required buckets exist.
func NewClient(cfg config.MinIOConfig) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio connect: %w", err)
	}

	client := &Client{
		mc:          mc,
		rawBucket:   cfg.RawBucket,
		hlsBucket:   cfg.HLSBucket,
		thumbBucket: cfg.ThumbnailsBucket,
		publicURL:   strings.TrimRight(cfg.PublicURL, "/"),
	}

	// Ensure buckets exist
	ctx := context.Background()
	for _, bucket := range []string{cfg.RawBucket, cfg.HLSBucket, cfg.ThumbnailsBucket} {
		exists, err := mc.BucketExists(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("check bucket %s: %w", bucket, err)
		}
		if !exists {
			if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return nil, fmt.Errorf("create bucket %s: %w", bucket, err)
			}
			log.Printf("✓ Created MinIO bucket: %s", bucket)
		}
	}

	// HLS playlists reference their variant playlists and segments by relative
	// path, so a player can only follow them if the objects are readable
	// without a signature. Raw uploads stay private.
	for _, bucket := range []string{cfg.HLSBucket, cfg.ThumbnailsBucket} {
		if err := mc.SetBucketPolicy(ctx, bucket, publicReadPolicy(bucket)); err != nil {
			return nil, fmt.Errorf("set public-read policy on %s: %w", bucket, err)
		}
	}

	log.Printf("✓ Connected to MinIO (%s)", cfg.Endpoint)
	return client, nil
}

// UploadFile uploads a file directly to the raw bucket bypassing presigned URLs.
func (c *Client) UploadFile(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.rawBucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("upload put: %w", err)
	}
	return nil
}

// PresignedUpload generates a presigned PUT URL for direct browser upload.// The instructor uploads raw video directly to MinIO, bypassing the API server.
func (c *Client) PresignedUpload(objectKey string, ttl time.Duration) (string, error) {
	presignedURL, err := c.mc.PresignedPutObject(
		context.Background(),
		c.rawBucket,
		objectKey,
		ttl,
	)
	if err != nil {
		return "", fmt.Errorf("presigned put: %w", err)
	}
	return presignedURL.String(), nil
}

// DownloadRawFile downloads a file from the raw bucket to a local path.
func (c *Client) DownloadRawFile(ctx context.Context, objectKey, filePath string) error {
	err := c.mc.FGetObject(ctx, c.rawBucket, objectKey, filePath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("download raw file: %w", err)
	}
	return nil
}

// UploadHLSFile uploads a file from a local path to the HLS bucket.
func (c *Client) UploadHLSFile(ctx context.Context, objectKey, filePath, contentType string) error {
	_, err := c.mc.FPutObject(ctx, c.hlsBucket, objectKey, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("upload hls file: %w", err)
	}
	return nil
}

// UploadThumbnail uploads a file from a local path to the thumbnails bucket.
func (c *Client) UploadThumbnail(ctx context.Context, objectKey, filePath, contentType string) error {
	_, err := c.mc.FPutObject(ctx, c.thumbBucket, objectKey, filePath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("upload thumbnail file: %w", err)
	}
	return nil
}

// PresignedDownload generates a presigned GET URL for video playback.
// Used until CloudFront is integrated.
func (c *Client) PresignedDownload(bucket, objectKey string, ttl time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := c.mc.PresignedGetObject(
		context.Background(),
		bucket,
		objectKey,
		ttl,
		reqParams,
	)
	if err != nil {
		return "", fmt.Errorf("presigned get: %w", err)
	}
	return presignedURL.String(), nil
}

// HLSPrefix is the object key prefix under which a module's HLS output lives.
func HLSPrefix(courseID, moduleID string) string {
	return fmt.Sprintf("courses/%s/%s", courseID, moduleID)
}

// HLSMasterURL returns the public URL of a module's HLS master playlist.
func (c *Client) HLSMasterURL(courseID, moduleID string) string {
	return fmt.Sprintf("%s/%s/%s/master.m3u8", c.publicURL, c.hlsBucket, HLSPrefix(courseID, moduleID))
}

// ThumbnailURL returns the public URL of an object in the thumbnails bucket.
func (c *Client) ThumbnailURL(objectKey string) string {
	return fmt.Sprintf("%s/%s/%s", c.publicURL, c.thumbBucket, objectKey)
}

func publicReadPolicy(bucket string) string {
	return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
}
