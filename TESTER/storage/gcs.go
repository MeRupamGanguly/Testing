package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type GCSProvider struct {
	client *storage.Client
}

func NewGCSProvider(ctx context.Context) (*GCSProvider, error) {
	client, err := storage.NewClient(ctx, storage.WithJSONReads())
	if err != nil {
		return nil, fmt.Errorf("gcs client init failed: %w", err)
	}
	return &GCSProvider{client: client}, nil
}

func (g *GCSProvider) Upload(ctx context.Context, bucket, object string, content io.Reader) error {
	writer := g.client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err := io.Copy(writer, content); err != nil {
		writer.Close()
		return fmt.Errorf("gcs upload failed: %w", err)
	}
	return writer.Close()
}

func (g *GCSProvider) Download(ctx context.Context, bucket, object string) (io.ReadCloser, error) {
	reader, err := g.client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs download failed: %w", err)
	}
	return reader, nil
}

func (g *GCSProvider) Delete(ctx context.Context, bucket, object string) error {
	err := g.client.Bucket(bucket).Object(object).Delete(ctx)
	if err != nil {
		return fmt.Errorf("gcs delete failed: %w", err)
	}
	return nil
}

func (g *GCSProvider) List(ctx context.Context, bucket, prefix string) ([]string, error) {
	it := g.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	var objects []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcs list failed: %w", err)
		}
		objects = append(objects, attrs.Name)
	}
	return objects, nil
}

func (g *GCSProvider) GetSignedURL(ctx context.Context, bucket, object string, expires time.Duration) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(expires),
	}

	url, err := g.client.Bucket(bucket).SignedURL(object, opts)
	if err != nil {
		return "", fmt.Errorf("gcs sign url failed: %w", err)
	}
	return url, nil
}
