package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Uploader interface {
	UploadFormFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error)
}

type MinioConfig struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	Bucket        string
	PublicBaseURL string
}

type MinioUploader struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

func NewMinioUploader(cfg MinioConfig) (*MinioUploader, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("minio endpoint is required")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, errors.New("minio credentials are required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("minio bucket is required")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if publicBaseURL == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		publicBaseURL = scheme + "://" + strings.TrimSpace(cfg.Endpoint) + "/" + cfg.Bucket
	}

	return &MinioUploader{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: publicBaseURL,
	}, nil
}

func (m *MinioUploader) UploadFormFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	if header == nil {
		return "", errors.New("file header is required")
	}

	extension := strings.ToLower(filepath.Ext(header.Filename))
	if extension == "" {
		extension = extensionFromContentType(header.Header.Get("Content-Type"))
	}
	objectName := generateObjectName(extension)

	contentType := header.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	size := header.Size
	if size <= 0 {
		data, err := io.ReadAll(file)
		if err != nil {
			return "", err
		}
		_, err = m.client.PutObject(ctx, m.bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			return "", err
		}
		return m.publicBaseURL + "/" + objectName, nil
	}

	_, err := m.client.PutObject(ctx, m.bucket, objectName, file, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}

	return m.publicBaseURL + "/" + objectName, nil
}

func extensionFromContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	default:
		return ".bin"
	}
}

func generateObjectName(extension string) string {
	buffer := make([]byte, 8)
	_, _ = rand.Read(buffer)
	return "media_" + time.Now().UTC().Format("20060102150405") + "_" + hex.EncodeToString(buffer) + extension
}
