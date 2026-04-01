package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Provider struct {
	client        *s3.Client
	presignClient *s3.PresignClient
}

func NewS3Provider(ctx context.Context, region string) (*S3Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("s3 config load failed: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(client)

	return &S3Provider{
		client:        client,
		presignClient: presignClient,
	}, nil
}

func (s *S3Provider) Upload(ctx context.Context, bucket, object string, content io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   content,
	})
	if err != nil {
		return fmt.Errorf("s3 upload failed: %w", err)
	}
	return nil
}

func (s *S3Provider) Download(ctx context.Context, bucket, object string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 download failed: %w", err)
	}
	return result.Body, nil
}

func (s *S3Provider) Delete(ctx context.Context, bucket, object string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return fmt.Errorf("s3 delete failed: %w", err)
	}
	return nil
}

func (s *S3Provider) List(ctx context.Context, bucket, prefix string) ([]string, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 list failed: %w", err)
	}

	var objects []string
	for _, item := range result.Contents {
		objects = append(objects, *item.Key)
	}
	return objects, nil
}

func (s *S3Provider) GetSignedURL(ctx context.Context, bucket, object string, expires time.Duration) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})
	if err != nil {
		return "", fmt.Errorf("s3 sign url failed: %w", err)
	}
	return req.URL, nil
}
