package storage

import (
	"context"
	"github.com/stretchr/testify/assert"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDirectoryHandler_EnsureDirExists(t *testing.T) {
	tempDir := t.TempDir()

	h, err := newLocalDir("test", config.LocalDirConfig{
		Path: tempDir,
		Mode: 0755,
	}, config.RetryConfig{Limit: 1}, tempDir, []string{".txt", ".log"})
	assert.NoError(t, err)

	// Test when directory already exists
	err = h.ensureDirExists(context.Background())
	assert.NoError(t, err)

	// Test when directory does not exist
	nonExistentDir := filepath.Join(tempDir, "newDir")
	h.dirConfig.Path = nonExistentDir
	err = h.ensureDirExists(context.Background())
	assert.NoError(t, err)

	// Verify directory was created
	_, exists := fsx.PathExists(nonExistentDir)
	assert.True(t, exists)
}

func TestLocalDirectoryHandler_SyncWithDir(t *testing.T) {
	tempDir := t.TempDir()
	destDir := filepath.Join(tempDir, "dest")
	defer func() {
		_ = os.RemoveAll(tempDir)
		_ = os.RemoveAll(destDir)
	}()
	srcFile := filepath.Join(tempDir, "source.txt")
	destFile := "destination.txt"

	// Create a source file
	err := os.WriteFile(srcFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	h, err := newLocalDir("test", config.LocalDirConfig{
		Path: destDir,
		Mode: 0755,
	}, config.RetryConfig{Limit: 1}, tempDir, []string{".txt", ".not-exist"})
	assert.NoError(t, err)

	// Test file synchronization
	uploadInfo, err := h.syncWithDir(context.Background(), srcFile, destFile)
	assert.NoError(t, err)
	assert.NotNil(t, uploadInfo)

	// Verify file was copied
	destPath := filepath.Join(destDir, destFile)
	_, exists := fsx.PathExists(destPath)
	assert.True(t, exists)

	// Test skipping copy if file already exists with the same checksum
	uploadInfo, err = h.syncWithDir(context.Background(), srcFile, destFile)
	assert.NoError(t, err)
	assert.NotNil(t, uploadInfo)
	assert.Equal(t, srcFile, uploadInfo.Src)
	assert.Equal(t, destPath, uploadInfo.Dest)
}
