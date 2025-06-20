package scanner

import (
	"context"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewScanner(t *testing.T) {
	// Test valid scanner creation
	s, err := newScanner("test-scanner", "/test/dir", ".txt", 10)
	assert.NoError(t, err)
	assert.NotNil(t, s)

	// Test invalid scanner creation with unsupported pattern
	s, err = newScanner("test-scanner", "/test/dir", "*.txt", 10)
	assert.Error(t, err)
	assert.Nil(t, s)
}

func TestScan(t *testing.T) {
	// Setup: Create a temporary directory with test files
	tempDir := t.TempDir()
	testFiles := []string{"file1.txt", "file2.log", "file3.txt"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644)
		assert.NoError(t, err)
	}

	// Initialize the scanner
	s, err := newScanner("test-scanner", tempDir, ".txt", 3)
	assert.NoError(t, err)

	// Create a context and error channel
	ctx := context.Background()
	errCh := make(chan error, 1)
	defer close(errCh)

	// Run the scanner
	results := s.Scan(ctx, errCh)

	// Verify the results
	expectedFiles := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file3.txt"),
	}

	var scannedFiles []string
	for result := range results {
		scannedFiles = append(scannedFiles, result.Path)
	}

	assert.ElementsMatch(t, expectedFiles, scannedFiles)
}

func TestScan_EmptyDirectory(t *testing.T) {
	// Setup: Create an empty temporary directory
	tempDir := t.TempDir()

	// Initialize the scanner
	s, err := NewScanner("test-scanner", tempDir, ".txt", 3)
	assert.NoError(t, err)

	// Create a context and error channel
	ctx := context.Background()
	errCh := make(chan error, 1)
	defer close(errCh)

	// Run the scanner
	results := s.Scan(ctx, errCh)

	// Verify no results are returned
	var scannedFiles []string
	for result := range results {
		scannedFiles = append(scannedFiles, result.Path)
	}
	assert.Empty(t, scannedFiles)
}

func TestScan_InvalidDirectory(t *testing.T) {
	// Initialize the scanner with a non-existent directory
	s, err := NewScanner("test-scanner", "/invalid/path", ".txt", 3)
	assert.NoError(t, err)

	// Create a context and error channel
	ctx := context.Background()
	errCh := make(chan error, 1)
	defer close(errCh)

	// Run the scanner
	results := s.Scan(ctx, errCh)

	// Verify no results are returned
	var scannedFiles []string
	for result := range results {
		scannedFiles = append(scannedFiles, result.Path)
	}
	assert.Empty(t, scannedFiles)
}
