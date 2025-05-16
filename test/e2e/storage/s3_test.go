package storage

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/storage"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"os"
	"path"
	"testing"
)

type s3TestHandler struct {
	dataDir      string
	configFile   string
	client       *minio.Client
	bucketConfig *config.BucketConfig
	retryConfig  *config.RetryConfig
}

func (h *s3TestHandler) loadConfig() error {
	if h.configFile == "" {
		h.configFile = path.Join(h.dataDir, "../config/.cheetah", "cheetah-test.yaml")
	}

	return config.Initialize(h.configFile)
}

func (h *s3TestHandler) initClient(t *testing.T, bucketConfig *config.BucketConfig, retryConfig *config.RetryConfig) error {
	h.bucketConfig = bucketConfig
	h.retryConfig = retryConfig

	// Setup S3 client
	var err error
	h.client, err = minio.New(bucketConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(bucketConfig.AccessKey, bucketConfig.SecretKey, ""),
		Secure: bucketConfig.UseSSL,
	})

	return err
}

func (h *s3TestHandler) verifyFile(t *testing.T, path string, cleanUp bool) {
	// Verify the uploaded file exists in the bucket
	_, name, _ := fsx.SplitFilePath(path)
	objectName := fsx.CombineFilePath(h.bucketConfig.Prefix, name, ".txt")
	attr, err := h.client.StatObject(context.Background(), h.bucketConfig.Bucket, objectName, minio.StatObjectOptions{})
	require.NoError(t, err)

	localChecksum, err := fsx.FileMD5(path)
	require.NoError(t, err)
	require.Equal(t, localChecksum, attr.ETag)

	// Cleanup
	if cleanUp {
		err = h.client.RemoveObject(context.Background(), h.bucketConfig.Bucket, objectName, minio.RemoveObjectOptions{})
		require.NoError(t, err)
	}
}

func (h *s3TestHandler) upload(t *testing.T, handler core.Storage) {
	// Create a temporary file
	tmpDir := path.Join(h.dataDir, "tmp")
	content := []byte("This is a test file for S3 integration testing.")

	existingFile := path.Join(tmpDir, "test-existing.txt")
	if _, exists := fsx.PathExists(existingFile); !exists {
		f, err := os.Create(existingFile)
		require.NoError(t, err)
		defer fsx.RemoveFile(existingFile)

		_, err = f.Write(content)
		require.NoError(t, err)
		fsx.CloseFile(f)
	}

	tempFile := path.Join(tmpDir, "test-file.txt")
	if _, exists := fsx.PathExists(tempFile); !exists {
		f, err := os.Create(tempFile)
		require.NoError(t, err)
		defer fsx.RemoveFile(tempFile)

		_, err = f.Write(content)
		require.NoError(t, err)
		fsx.CloseFile(f)
	}

	// Prepare the ScannerResult
	stored := make(chan core.StorageResult)

	// Test Put method
	go func() {
		defer close(stored)
		handler.Put(context.Background(), core.ScannerResult{Path: tempFile}, stored)
		handler.Put(context.Background(), core.ScannerResult{Path: existingFile}, stored)
	}()

	// Verify the result
	for result := range stored {
		require.NoError(t, result.Error)
	}

	// Verify the uploaded file exists in the bucket
	h.verifyFile(t, existingFile, true)
	h.verifyFile(t, tempFile, false)
}

func TestS3_Put(t *testing.T) {
	dataDir := "../../data"
	var err error
	h := &s3TestHandler{dataDir: dataDir}
	err = h.loadConfig()
	require.NoError(t, err)

	pipeline := config.Get().Pipelines[0]

	err = h.initClient(t, pipeline.Processor.Storage.S3, pipeline.Processor.Retry)
	require.NoError(t, err)

	handler, err := storage.NewS3("s3-1", *h.bucketConfig, *h.retryConfig, []string{".txt"})
	require.NoError(t, err)

	h.upload(t, handler)
}

func TestGCSWithS3_Put(t *testing.T) {
	dataDir := "../../data"
	var err error
	h := &s3TestHandler{dataDir: dataDir}
	err = h.loadConfig()
	require.NoError(t, err)

	pipeline := config.Get().Pipelines[0]

	err = h.initClient(t, pipeline.Processor.Storage.GCS, pipeline.Processor.Retry)
	require.NoError(t, err)

	handler, err := storage.NewGCSWithS3("gcs-1", *h.bucketConfig, *h.retryConfig, []string{".txt"})
	require.NoError(t, err)

	h.upload(t, handler)
}
