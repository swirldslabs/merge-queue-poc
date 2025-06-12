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
	filesDir     string
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

func (h *s3TestHandler) verifyFile(t *testing.T, filePath string, cleanUp bool) {
	// Verify the uploaded file exists in the bucket
	_, name, _ := fsx.SplitFilePath(filePath)
	objectName := fsx.CombineFilePath(h.bucketConfig.Prefix, name, ".txt")
	attr, err := h.client.StatObject(context.Background(), h.bucketConfig.Bucket, objectName, minio.StatObjectOptions{})
	require.NoError(t, err)

	localChecksum, err := fsx.FileMD5(filePath)
	require.NoError(t, err)
	require.Equal(t, localChecksum, attr.ETag)

	// Cleanup
	if cleanUp {
		err = h.client.RemoveObject(context.Background(), h.bucketConfig.Bucket, objectName, minio.RemoveObjectOptions{})
		require.NoError(t, err)
	}
}

func (h *s3TestHandler) createMockFile(t *testing.T, filePath string) string {
	content := []byte("This is a test file for S3 integration testing.")
	if _, exists := fsx.PathExists(filePath); !exists {
		f, err := os.Create(filePath)
		require.NoError(t, err)

		_, err = f.Write(content)
		require.NoError(t, err)
		fsx.CloseFile(f)
	}

	return filePath
}
func (h *s3TestHandler) upload(t *testing.T, handler core.Storage, files []string) <-chan core.StorageResult {

	// Prepare the ScannerResult
	stored := make(chan core.StorageResult)

	// Test Put method
	go func() {
		defer close(stored)
		for _, file := range files {
			handler.Put(context.Background(), core.ScannerResult{Path: file}, stored)
		}
	}()

	return stored
}

func TestS3_Put_Missing_File_Error(t *testing.T) {
	dataDir := "../../data"
	filesDir := path.Join(dataDir, "tmp")

	var err error
	h := &s3TestHandler{dataDir: dataDir, filesDir: filesDir}
	err = h.loadConfig()
	require.NoError(t, err)

	pipeline := config.Get().Pipelines[0]

	err = h.initClient(t, pipeline.Processor.Storage.S3, pipeline.Processor.Retry)
	require.NoError(t, err)

	handler, err := storage.NewS3("s3-1", *h.bucketConfig, *h.retryConfig, filesDir, []string{".txt"})
	require.NoError(t, err)

	file1 := h.createMockFile(t, path.Join(h.filesDir, "file1.txt"))
	file2 := h.createMockFile(t, path.Join(h.filesDir, "file2.txt"))
	files := []string{file1, file2, path.Join(h.filesDir, "missing.txt")}

	stored := h.upload(t, handler, files)

	// Verify the result
	found := false
	for result := range stored {
		if result.Error != nil {
			require.Contains(t, result.Error.Error(), "candidate file is missing")
			found = true
		} else {
			require.True(t, result.Src != files[2], "Expected error for missing file, but got result: %v", result)
		}
	}
	require.True(t, found)

	// Verify the uploaded file exists in the bucket
	for _, file := range []string{file1, file2} {
		h.verifyFile(t, file, true)
		fsx.RemoveFile(file)
	}
}

func TestS3_Put(t *testing.T) {
	dataDir := "../../data"
	filesDir := path.Join(dataDir, "tmp")

	var err error
	h := &s3TestHandler{dataDir: dataDir, filesDir: filesDir}
	err = h.loadConfig()
	require.NoError(t, err)

	pipeline := config.Get().Pipelines[0]

	err = h.initClient(t, pipeline.Processor.Storage.S3, pipeline.Processor.Retry)
	require.NoError(t, err)

	handler, err := storage.NewS3("s3-1", *h.bucketConfig, *h.retryConfig, filesDir, []string{".txt"})
	require.NoError(t, err)

	file1 := h.createMockFile(t, path.Join(h.filesDir, "file1.txt"))
	file2 := h.createMockFile(t, path.Join(h.filesDir, "file2.txt"))
	files := []string{file1, file2}

	stored := h.upload(t, handler, files)

	// Verify the result
	for result := range stored {
		require.NoError(t, result.Error)
	}

	// Verify the uploaded file exists in the bucket
	for _, file := range files {
		h.verifyFile(t, file, true)
		fsx.RemoveFile(file)
	}
}

func TestGCS_Put(t *testing.T) {
	dataDir := "../../data"
	filesDir := path.Join(dataDir, "tmp")

	var err error
	h := &s3TestHandler{dataDir: dataDir, filesDir: filesDir}
	err = h.loadConfig()
	require.NoError(t, err)

	pipeline := config.Get().Pipelines[0]

	err = h.initClient(t, pipeline.Processor.Storage.GCS, pipeline.Processor.Retry)
	require.NoError(t, err)

	handler, err := storage.NewGCSWithS3("gcs-1", *h.bucketConfig, *h.retryConfig, filesDir, []string{".txt"})
	require.NoError(t, err)

	file1 := h.createMockFile(t, path.Join(h.filesDir, "file1.txt"))
	file2 := h.createMockFile(t, path.Join(h.filesDir, "file2.txt"))
	files := []string{file1, file2}

	stored := h.upload(t, handler, files)

	// Verify the result
	for result := range stored {
		require.NoError(t, result.Error)
	}

	// Verify the uploaded file exists in the bucket
	for _, file := range files {
		h.verifyFile(t, file, true)
		fsx.RemoveFile(file)
	}
}
