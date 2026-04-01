package storage

import (
	"context"
	"io"
	"time"

	"github.com/stretchr/testify/mock"
)

type MockStorageProvider struct {
	mock.Mock
}

func (m *MockStorageProvider) Upload(ctx context.Context, bucket, object string, content io.Reader) error {
	args := m.Called(ctx, bucket, object, content)
	return args.Error(0)
}

func (m *MockStorageProvider) Download(ctx context.Context, bucket, object string) (io.ReadCloser, error) {
	args := m.Called(ctx, bucket, object)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorageProvider) Delete(ctx context.Context, bucket, object string) error {
	args := m.Called(ctx, bucket, object)
	return args.Error(0)
}

func (m *MockStorageProvider) List(ctx context.Context, bucket, prefix string) ([]string, error) {
	args := m.Called(ctx, bucket, prefix)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockStorageProvider) GetSignedURL(ctx context.Context, bucket, object string, expires time.Duration) (string, error) {
	args := m.Called(ctx, bucket, object, expires)
	return args.String(0), args.Error(1)
}

func (m *MockStorageProvider) Modify(ctx context.Context, bucket, object string, content io.Reader) error {
	args := m.Called(ctx, bucket, object, content)
	return args.Error(0)
}
