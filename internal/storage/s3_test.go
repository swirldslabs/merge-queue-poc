package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"os"
	"path/filepath"
	"testing"
)

// mockS3Client is a mock implementation of the s3Client interface.
type mockS3Client struct {
	mock.Mock
}

func (m *mockS3Client) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	args := m.Called(ctx, bucketName)
	return args.Bool(0), args.Error(1)
}

func (m *mockS3Client) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	args := m.Called(ctx, bucketName, opts)
	return args.Error(0)
}

func (m *mockS3Client) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(minio.ObjectInfo), args.Error(1)
}

func (m *mockS3Client) FPutObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, filePath, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func TestS3Handler_EnsureBucketExists(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	mockClient := new(mockS3Client)
	bucketName := "test-bucket"
	bucketConfig := config.BucketConfig{Bucket: bucketName, Region: "us-east-1"}

	h := &s3Handler{
		handler: &handler{
			id:             "s3-handler",
			storageType:    TypeS3,
			fileExtensions: []string{".txt", ".log"},
			pathPrefix:     bucketConfig.Prefix,
			rootDir:        tempDir,
		},
		client:       mockClient,
		bucketConfig: bucketConfig,
		retryConfig:  config.RetryConfig{Limit: 1},
		bucketExists: make(map[string]bool),
	}

	// Test case: Bucket already exists
	mockClient.On("BucketExists", mock.Anything, bucketName).Return(true, nil).Once()
	err := h.ensureBucketExists(context.Background())
	assert.NoError(t, err)
	assert.True(t, h.bucketExists[bucketName])

	// Test case: Bucket does not exist, creation succeeds
	mockClient.On("BucketExists", mock.Anything, bucketName).Return(false, nil).Once()
	mockClient.On("MakeBucket", mock.Anything, bucketName, mock.Anything).Return(nil).Once()
	h.bucketExists = make(map[string]bool) // Resetting the bucket existence cache
	err = h.ensureBucketExists(context.Background())
	assert.NoError(t, err)
	assert.True(t, h.bucketExists[bucketName])

	// Test case: Bucket creation fails
	mockClient.On("BucketExists", mock.Anything, bucketName).Return(false, nil).Once()
	mockClient.On("MakeBucket", mock.Anything, bucketName, mock.Anything).Return(errors.New("creation failed")).Once()
	h.bucketExists = make(map[string]bool) // Resetting the bucket existence cache
	err = h.ensureBucketExists(context.Background())
	assert.Error(t, err)
	assert.False(t, h.bucketExists[bucketName])
}

func TestS3Handler_SyncWithBucket(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	mockClient := new(mockS3Client)
	bucketName := "test-bucket"
	bucketConfig := config.BucketConfig{Bucket: bucketName}
	h := &s3Handler{
		handler: &handler{
			id:             "s3-handler",
			storageType:    TypeS3,
			fileExtensions: []string{".txt", ".log"},
			pathPrefix:     bucketConfig.Prefix,
			rootDir:        tempDir,
		},
		client:       mockClient,
		bucketConfig: bucketConfig,
		retryConfig:  config.RetryConfig{Limit: 1},
		bucketExists: make(map[string]bool),
	}

	srcFile := filepath.Join(tempDir, "source.txt")

	// Create a source file
	err := os.WriteFile(srcFile, []byte("test content"), 0644)
	assert.NoError(t, err)
	localChecksum, err := fsx.FileMD5(srcFile)
	assert.NoError(t, err)

	objectName := "object.txt"

	// Test case: File already exists in bucket with the same checksum
	mockClient.On("StatObject", mock.Anything, bucketName, objectName, mock.Anything).Return(minio.ObjectInfo{
		ETag: localChecksum,
		Key:  objectName,
	}, nil).Once()
	info, err := h.syncWithBucket(context.Background(), srcFile, objectName)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, objectName, info.Dest)

	// Test case: File upload succeeds
	mockClient.On("StatObject", mock.Anything, bucketName, objectName, mock.Anything).Return(minio.ObjectInfo{}, fmt.Errorf("not found")).Once()
	mockClient.On("FPutObject", mock.Anything, bucketName, objectName, srcFile, mock.Anything).Return(minio.UploadInfo{
		ETag: localChecksum,
		Key:  objectName,
	}, nil).Once()
	info, err = h.syncWithBucket(context.Background(), srcFile, objectName)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, objectName, info.Dest)

	// Test case: File upload fails
	mockClient.On("StatObject", mock.Anything, bucketName, objectName, mock.Anything).Return(minio.ObjectInfo{}, fmt.Errorf("not found")).Once()
	mockClient.On("FPutObject", mock.Anything, bucketName, objectName, srcFile, mock.Anything).Return(minio.UploadInfo{}, errors.New("upload failed")).Once()
	info, err = h.syncWithBucket(context.Background(), srcFile, objectName)
	assert.Error(t, err)
	assert.Nil(t, info)

	// Test case: File upload fails because of checksum mismatch
	mockClient.On("StatObject", mock.Anything, bucketName, objectName, mock.Anything).Return(minio.ObjectInfo{}, fmt.Errorf("not found")).Once()
	mockClient.On("FPutObject", mock.Anything, bucketName, objectName, srcFile, mock.Anything).Return(minio.UploadInfo{
		ETag: "invalid",
		Key:  objectName,
	}, nil).Once()
	info, err = h.syncWithBucket(context.Background(), srcFile, objectName)
	assert.Error(t, err)
	assert.Nil(t, info)
}
