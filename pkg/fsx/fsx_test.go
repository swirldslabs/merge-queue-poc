package fsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathExists(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	existingFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(existingFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Test existing file
	_, exists := PathExists(existingFile)
	assert.True(t, exists)

	// Test non-existing file
	_, exists = PathExists(filepath.Join(tempDir, "nonexistent.txt"))
	assert.False(t, exists)
}

func TestCopy(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	srcFile := filepath.Join(tempDir, "source.txt")
	destFile := filepath.Join(tempDir, "destination.txt")

	// Create source file
	err := os.WriteFile(srcFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Test copying file
	err = Copy(srcFile, destFile, 0644)
	assert.NoError(t, err)

	// Verify destination file exists and content matches
	content, err := os.ReadFile(destFile)
	assert.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestMove(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	srcFile := filepath.Join(tempDir, "source.txt")
	destFile := filepath.Join(tempDir, "destination.txt")

	// Create source file
	err := os.WriteFile(srcFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Test moving file
	err = Move(srcFile, destFile, 0644)
	assert.NoError(t, err)

	// Verify source file no longer exists
	_, exists := PathExists(srcFile)
	assert.False(t, exists)

	// Verify destination file exists and content matches
	content, err := os.ReadFile(destFile)
	assert.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

func TestFileMD5(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	testFile := filepath.Join(tempDir, "test.txt")

	// Create test file
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Test MD5 hash
	hash, err := FileMD5(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "9473fdd0d880a43c21b7778d34872157", hash)
}

func TestFileSha256(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	testFile := filepath.Join(tempDir, "test.txt")

	// Create test file
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Test SHA256 hash
	hash, err := FileSha256(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72", hash)
}
