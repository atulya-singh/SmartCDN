package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/at/smartcdn/internal/config"
)

type Storage struct {
	client *minio.Client
	bucket string
}

func NewStorage(ctx context.Context, cfg *config.Config) (*Storage, error) {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioSecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket %q: %w", cfg.MinioBucket, err)
		}
		slog.Info("created bucket", "bucket", cfg.MinioBucket)
	}

	return &Storage{client: client, bucket: cfg.MinioBucket}, nil
}

func (s *Storage) Upload(ctx context.Context, filename string, data io.Reader, contentType string) (string, error) {
	id := uuid.New().String()
	objectName := id + "/" + filename

	buf, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("failed to read upload data: %w", err)
	}

	_, err = s.client.PutObject(ctx, s.bucket, objectName, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"original-filename": filename,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to minio: %w", err)
	}

	return id, nil
}

// Ping checks MinIO connectivity by listing buckets.
func (s *Storage) Ping(ctx context.Context) error {
	_, err := s.client.ListBuckets(ctx)
	return err
}

func (s *Storage) Download(ctx context.Context, imageID string) ([]byte, string, error) {
	// List objects with the imageID prefix to find the stored file.
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    imageID + "/",
		Recursive: false,
	})

	var objectName string
	for obj := range objectCh {
		if obj.Err != nil {
			return nil, "", fmt.Errorf("failed to list objects for %q: %w", imageID, obj.Err)
		}
		objectName = obj.Key
		break
	}
	if objectName == "" {
		return nil, "", fmt.Errorf("image %q not found", imageID)
	}

	obj, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object from minio: %w", err)
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat object: %w", err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read object data: %w", err)
	}

	return data, info.ContentType, nil
}
