# TestingHere is the complete project architecture and production-ready Go code for your Object Storage Abstraction.
This implementation covers AWS S3, Google Cloud Storage (GCS), and Azure Blob Storage, prioritizing the exact interfaces and Pre-signed URL generation you requested.
1. Project Architecture
Create the following directory structure for your project:
'''bash
storage-abstraction/
├── go.mod
├── main.go             # Example usage
└── storage/
    ├── errors.go       # Domain-specific error mapping
    ├── factory.go      # Initialization logic
    ├── interface.go    # Core ObjectStore contract
    ├── s3.go           # AWS S3 implementation
    ├── gcs.go          # Google Cloud Storage implementation
    └── azure.go        # Azure Blob Storage implementation
'''
To initialize the project and get the required dependencies, run:
go mod init storage-abstraction
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/s3
go get cloud.google.com/go/storage
go get github.com/Azure/azure-sdk-for-go/sdk/storage/azblob

2. Core Abstraction (storage/interface.go & storage/errors.go)
This defines the contract all providers must follow, alongside standard errors to ensure your application code doesn't need to know which cloud provider is throwing the error.
storage/errors.go
'''go
package storage

import "errors"

var (
	ErrObjectNotFound     = errors.New("object not found")
	ErrProviderNotSupport = errors.New("storage provider not supported")
	ErrInvalidMethod      = errors.New("invalid HTTP method for signed URL")
)

storage/interface.go
package storage

import (
	"context"
	"io"
	"time"
)

// ObjectStore defines the unified interface for cloud storage
type ObjectStore interface {
	Put(ctx context.Context, key string, data io.Reader, contentType string) error
	Get(ctx context.Context, key string, dst io.Writer) error
	Delete(ctx context.Context, key string) error
	// GetSignedURL generates a pre-signed URL. method should be "GET" or "PUT".
	GetSignedURL(ctx context.Context, key string, method string, expires time.Duration) (string, error)
}

3. The Factory (storage/factory.go)
The factory initializes the correct implementation based on configuration.
package storage

import (
	"context"
	"fmt"
)

// Config holds the necessary configuration for all providers.
// In a real app, populate this via env vars or a config file.
type Config struct {
	Provider          string // "s3", "gcs", "azure"
	BucketOrContainer string
	Region            string // For S3
	AzureAccountName  string // For Azure SAS
	AzureAccountKey   string // For Azure SAS
}

// NewObjectStore returns the concrete implementation of ObjectStore
func NewObjectStore(ctx context.Context, cfg Config) (ObjectStore, error) {
	switch cfg.Provider {
	case "s3":
		return NewS3Store(ctx, cfg.Region, cfg.BucketOrContainer)
	case "gcs":
		return NewGCSStore(ctx, cfg.BucketOrContainer)
	case "azure":
		return NewAzureStore(cfg.AzureAccountName, cfg.AzureAccountKey, cfg.BucketOrContainer)
	default:
		return nil, fmt.Errorf("%w: %s", ErrProviderNotSupport, cfg.Provider)
	}
}

4. Cloud Implementations
AWS S3 (storage/s3.go)
Uses the modern AWS SDK v2, utilizing the dedicated PresignClient.
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

type S3Store struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

func NewS3Store(ctx context.Context, region, bucket string) (*S3Store, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(client)

	return &S3Store{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
	}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, data io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3Store) Get(ctx context.Context, key string, dst io.Writer) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err // In production, check for s3.ErrCodeNoSuchKey and map to ErrObjectNotFound
	}
	defer out.Body.Close()
	_, err = io.Copy(dst, out.Body)
	return err
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Store) GetSignedURL(ctx context.Context, key string, method string, expires time.Duration) (string, error) {
	var req *s3.PresignedHTTPRequest
	var err error

	switch method {
	case "GET":
		req, err = s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(expires))
	case "PUT":
		req, err = s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(expires))
	default:
		return "", ErrInvalidMethod
	}

	if err != nil {
		return "", fmt.Errorf("failed to sign request: %w", err)
	}
	return req.URL, nil
}

Google Cloud Storage (storage/gcs.go)
Uses the standard cloud.google.com/go/storage package. Note: GCS signed URLs require service account credentials.
package storage

import (
	"context"
	"io"
	"time"

	"cloud.google.com/go/storage"
)

type GCSStore struct {
	client *storage.Client
	bucket string
}

func NewGCSStore(ctx context.Context, bucket string) (*GCSStore, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCSStore{
		client: client,
		bucket: bucket,
	}, nil
}

func (g *GCSStore) Put(ctx context.Context, key string, data io.Reader, contentType string) error {
	obj := g.client.Bucket(g.bucket).Object(key)
	w := obj.NewWriter(ctx)
	w.ContentType = contentType

	if _, err := io.Copy(w, data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (g *GCSStore) Get(ctx context.Context, key string, dst io.Writer) error {
	r, err := g.client.Bucket(g.bucket).Object(key).NewReader(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return ErrObjectNotFound
		}
		return err
	}
	defer r.Close()

	_, err = io.Copy(dst, r)
	return err
}

func (g *GCSStore) Delete(ctx context.Context, key string) error {
	err := g.client.Bucket(g.bucket).Object(key).Delete(ctx)
	if err == storage.ErrObjectNotExist {
		return ErrObjectNotFound
	}
	return err
}

func (g *GCSStore) GetSignedURL(ctx context.Context, key string, method string, expires time.Duration) (string, error) {
	opts := &storage.SignedURLOptions{
		Method:  method,
		Expires: time.Now().Add(expires),
	}
	// Note: If running on GCP Workload Identity without a direct Service Account JSON,
	// you may need to use storage.SignBytes or provide GoogleAccessID/PrivateKey in opts.
	return g.client.Bucket(g.bucket).SignedURL(key, opts)
}

Azure Blob Storage (storage/azure.go)
Uses the modern azblob module. Azure requires creating a Shared Access Signature (SAS) token for Pre-signed URLs.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

type AzureStore struct {
	client        *azblob.Client
	sharedKeyCred *azblob.SharedKeyCredential
	container     string
}

func NewAzureStore(accountName, accountKey, container string) (*AzureStore, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
	client, err := azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, err
	}

	return &AzureStore{
		client:        client,
		sharedKeyCred: cred,
		container:     container,
	}, nil
}

func (a *AzureStore) Put(ctx context.Context, key string, data io.Reader, contentType string) error {
	// Using UploadStream to handle files of unknown size directly from io.Reader
	_, err := a.client.UploadStream(ctx, a.container, key, data, &azblob.UploadStreamOptions{
		HTTPHeaders: &azblob.BlobHTTPHeaders{BlobContentType: &contentType},
	})
	return err
}

func (a *AzureStore) Get(ctx context.Context, key string, dst io.Writer) error {
	resp, err := a.client.DownloadStream(ctx, a.container, key, nil)
	if err != nil {
		return err // In production, check for Azure Storage error codes to map to ErrObjectNotFound
	}
	defer resp.Body.Close()

	_, err = io.Copy(dst, resp.Body)
	return err
}

func (a *AzureStore) Delete(ctx context.Context, key string) error {
	_, err := a.client.DeleteBlob(ctx, a.container, key, nil)
	return err
}

func (a *AzureStore) GetSignedURL(ctx context.Context, key string, method string, expires time.Duration) (string, error) {
	perms := sas.BlobPermissions{}
	if method == "GET" {
		perms.Read = true
	} else if method == "PUT" {
		perms.Write = true
		perms.Create = true
	} else {
		return "", ErrInvalidMethod
	}

	sasQueryParams, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     time.Now().UTC(),
		ExpiryTime:    time.Now().UTC().Add(expires),
		Permissions:   perms.String(),
		ContainerName: a.container,
		BlobName:      key,
	}.SignWithSharedKey(a.sharedKeyCred)

	if err != nil {
		return "", err
	}

	// Construct the final URL
	blobURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s?%s",
		a.sharedKeyCred.AccountName(), a.container, key, sasQueryParams.Encode())
	return blobURL, nil
}

5. Putting it together (main.go)
Here is how your application layer will actually call this abstraction.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"storage-abstraction/storage"
)

func main() {
	ctx := context.Background()

	// Example config - in a real app, read from Viper/Env
	cfg := storage.Config{
		Provider:          "s3", // change to "gcs" or "azure" to test
		BucketOrContainer: "my-app-bucket",
		Region:            "us-east-1", 
	}

	// Initialize the store via the Factory
	store, err := storage.NewObjectStore(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Generate a Presigned GET URL valid for 15 minutes
	objectKey := "images/profile.png"
	url, err := store.GetSignedURL(ctx, objectKey, "GET", 15*time.Minute)
	if err != nil {
		log.Fatalf("Failed to generate presigned URL: %v", err)
	}

	fmt.Printf("Temporary access URL: %s\n", url)
}
'''
Would you like me to walk through how to mock this ObjectStore interface using a tool like Mockery so you can write unit tests for your application code?
