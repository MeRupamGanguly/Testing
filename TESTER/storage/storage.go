package storage

import (
	"context"
	"io"
	"os"
	"time"
)

type StorageProvider interface {
	Upload(ctx context.Context, bucket, object string, content io.Reader) error
	Download(ctx context.Context, bucket, object string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, object string) error
	List(ctx context.Context, bucket, prefix string) ([]string, error)
	GetSignedURL(ctx context.Context, bucket, object string, expires time.Duration) (string, error)
}

type StorageService struct {
	storage StorageProvider
}

func NewStorage(ctx context.Context) (provider StorageProvider, err error) {
	cloudEnv := os.Getenv("CLOUD_PROVIDER")
	if cloudEnv == "AWS" {
		region := os.Getenv("REGION")
		provider, err = NewS3Provider(ctx, region)
	} else {
		provider, err = NewGCSProvider(ctx)
	}
	if err != nil {
		return nil, err
	}
	return provider, nil
}
