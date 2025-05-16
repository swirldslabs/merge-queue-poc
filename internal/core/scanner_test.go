package core

import (
	"context"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	// Setup: Create a temporary directory with test files
	tempDir := t.TempDir()
	testFiles := []string{"file1.txt", "file2.log", "file3.txt"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644)
		assert.NoError(t, err)
	}

	// Initialize the scanner
	s, err := NewScanner("test-scanner", tempDir, ".txt", 3)
	assert.NoError(t, err)

	// Create a context and error channel
	ctx := context.Background()
	errCh := make(chan error, 1)

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

	// Ensure no errors were reported
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	default:
		// No errors
	}
}
