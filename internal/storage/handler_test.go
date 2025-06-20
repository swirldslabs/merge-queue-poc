package storage

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.hedera.com/solo-cheetah/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_handler_Put(t *testing.T) {
	// Setup: Create a temporary directory and files
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	markerFile := filepath.Join(tempDir, "file.mf")
	err := os.WriteFile(markerFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	dataFile := filepath.Join(tempDir, "file.data")
	err = os.WriteFile(dataFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Mock syncFile function
	mockSyncFile := func(ctx context.Context, src string, dest string) (*core.UploadInfo, error) {
		return &core.UploadInfo{Src: src, Dest: dest}, nil
	}

	// Create a handler instance
	h := &handler{
		id:          "test-handler",
		storageType: "local",
		rootDir:     tempDir,
		pathPrefix:  "uploads",
		syncFile:    mockSyncFile,
	}

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan core.StorageResult, 1)

	// Execute the Put function
	go func() {
		defer close(stored)
		h.Put(ctx, core.ScannerResult{Path: markerFile}, []string{dataFile, markerFile}, stored)
	}()

	// Verify the result
	for result := range stored {
		require.NoError(t, result.Error)
	}
}

func Test_handler_Put_Missing_File(t *testing.T) {
	// Setup: Create a temporary directory and files
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	markerFile := filepath.Join(tempDir, "file.mf")
	err := os.WriteFile(markerFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Mock syncFile function
	mockSyncFile := func(ctx context.Context, src string, dest string) (*core.UploadInfo, error) {
		return &core.UploadInfo{Src: src, Dest: dest}, nil
	}

	// Create a handler instance
	h := &handler{
		id:          "test-handler",
		storageType: "local",
		rootDir:     tempDir,
		pathPrefix:  "uploads",
		syncFile:    mockSyncFile,
	}

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan core.StorageResult, 1)

	// Execute the Put function
	go func() {
		defer close(stored)
		h.Put(ctx, core.ScannerResult{Path: markerFile}, []string{markerFile, filepath.Join(tempDir, "file.data")}, stored)
	}()

	// Verify the result
	for result := range stored {
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "candidate file is missing")
		assert.Contains(t, result.MarkerPath, "file.mf")
	}
}

func Test_handler_Put_Upload_Failed(t *testing.T) {
	// Setup: Create a temporary directory and files
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	markerFile := filepath.Join(tempDir, "file.mf")
	err := os.WriteFile(markerFile, []byte("test content"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "file.data"), []byte("test content"), 0644)
	assert.NoError(t, err)

	// Mock syncFile function
	mockSyncFile := func(ctx context.Context, src string, dest string) (*core.UploadInfo, error) {
		if strings.HasSuffix(src, ".mf") {
			return &core.UploadInfo{Src: src, Dest: dest}, nil
		}
		return nil, fmt.Errorf("failed to upload file: %s", src)
	}

	// Create a handler instance
	h := &handler{
		id:          "test-handler",
		storageType: "local",
		rootDir:     tempDir,
		pathPrefix:  "uploads",
		syncFile:    mockSyncFile,
	}

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan core.StorageResult, 1)

	// Execute the Put function
	go func() {
		defer close(stored)
		h.Put(ctx, core.ScannerResult{Path: markerFile}, []string{markerFile, filepath.Join(tempDir, "file.data")}, stored)
	}()

	// Verify the result
	for result := range stored {
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "failed to upload file")
		assert.Contains(t, result.MarkerPath, "file.mf")
	}
}
