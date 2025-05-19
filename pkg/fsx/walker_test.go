package fsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalker_Start(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test directory structure
	err := os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "subdir", "file2.txt"), []byte("content2"), 0644)
	assert.NoError(t, err)

	walker := NewWalker(1)

	// Collect visited paths
	var visited []string
	err = walker.Start(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		visited = append(visited, path)
		return nil
	})
	assert.NoError(t, err)

	// Verify all files and directories were visited
	assert.Equal(t, 4, len(visited))
	assert.Contains(t, visited, tempDir)
	assert.Contains(t, visited, filepath.Join(tempDir, "subdir"))
	assert.Contains(t, visited, filepath.Join(tempDir, "file1.txt"))
	assert.Contains(t, visited, filepath.Join(tempDir, "subdir", "file2.txt"))
}

func TestWalker_readDirEntries(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"), 0644)
	assert.NoError(t, err)

	walker := NewWalker(1)

	// Read directory entries
	names, err := walker.readDirEntries(tempDir, 1)
	assert.NoError(t, err)
	assert.Len(t, names, 1)

	// Read remaining entries
	names, err = walker.readDirEntries(tempDir, 1)
	assert.NoError(t, err)
	assert.Len(t, names, 1)

	// No more entries
	names, err = walker.readDirEntries(tempDir, 1)
	assert.NoError(t, err)
	assert.Empty(t, names)
}

func TestWalker_getOrOpenFile(t *testing.T) {
	tempDir := t.TempDir()

	walker := NewWalker(1)

	// Open a directory
	file, err := walker.getOrOpenFile(tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, file)

	// Retrieve the same file from the map
	file2, err := walker.getOrOpenFile(tempDir)
	assert.NoError(t, err)
	assert.Equal(t, file, file2)

	// Cleanup
	err = walker.closeFile(tempDir, nil)
	assert.NoError(t, err)
}

func TestWalker_End(t *testing.T) {
	tempDir := t.TempDir()

	walker := NewWalker(1)

	// Open a directory
	_, err := walker.getOrOpenFile(tempDir)
	assert.NoError(t, err)

	// End the walker and ensure resources are released
	walker.End()
	assert.Empty(t, walker.opened)
}
