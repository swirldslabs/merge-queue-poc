package core

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
	"testing"
)

type mockStorage struct {
	id          string
	storageType string
	putFunc     func(ctx context.Context, item ScannerResult, stored chan<- StorageResult)
}

func (m *mockStorage) Info() string {
	return m.id
}

func (m *mockStorage) Type() string {
	return m.storageType
}

func (m *mockStorage) Put(ctx context.Context, item ScannerResult, stored chan<- StorageResult) {
	if m.putFunc != nil {
		m.putFunc(ctx, item, stored)
	} else {
		// Default behavior for testing
		stored <- StorageResult{
			Error:   nil,
			Src:     item.Path,
			Dest:    []string{"mock/destination"},
			Type:    m.storageType,
			Handler: m.id,
		}

		logx.As().Info().Msg(fmt.Sprintf("Mock storage %s processed file %s", m.id, item.Path))
	}
}

func TestProcess_Upload_Success(t *testing.T) {
	// Setup: Create a temporary file to simulate a scanned file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "file1.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	items := make(chan ScannerResult, 2)
	testFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644)
		assert.NoError(t, err)
		select {
		case items <- ScannerResult{Path: filepath.Join(tempDir, file)}:
		}
	}
	close(items)

	// Execute the upload function
	ctx := context.Background()
	storages := []Storage{
		&mockStorage{id: "mock-storage-1", storageType: "S3"},
	}
	fileExtensions := []string{".txt"}
	p, err := newProcessor("test-processor", storages, fileExtensions)
	assert.NoError(t, err)
	stored := p.upload(ctx, items)

	// Verify the result
	var results []ProcessorResult
	for result := range stored {
		assert.NoError(t, result.Error)
		results = append(results, result)
		ext := filepath.Ext(result.Path)
		assert.Contains(t, fileExtensions, ext)
	}

	// Check if the result contains the expected storage type
	assert.Equal(t, 2, len(results))
}

func TestProcess_Upload_Failure(t *testing.T) {
	// Setup: Create a temporary file to simulate a scanned file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "file1.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	items := make(chan ScannerResult, 3)
	testFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0644)
		assert.NoError(t, err)
		select {
		case items <- ScannerResult{Path: filepath.Join(tempDir, file)}:
		}
	}

	// add an invalid file, which should be ignored
	items <- ScannerResult{Path: filepath.Join(tempDir, "invalid.txt")}
	close(items)

	// Create a context and result channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Execute the upload function
	storages := []Storage{
		&mockStorage{id: "mock-storage-1", storageType: "S3", putFunc: func(ctx context.Context, item ScannerResult, stored chan<- StorageResult) {
			// Default behavior for testing
			stored <- StorageResult{
				Error:   fmt.Errorf("failed to upload"),
				Src:     item.Path,
				Dest:    []string{},
				Type:    "S3",
				Handler: "mock-storage-1",
			}
		}},
	}
	fileExtensions := []string{".txt"}
	p, err := newProcessor("test-processor", storages, fileExtensions)
	assert.NoError(t, err)
	stored := p.upload(ctx, items)

	// Verify the result
	var totalResults int
	for result := range stored {
		totalResults++
		assert.Error(t, result.Error)
	}

	assert.Equal(t, 2, totalResults)
}

func TestProcessor_PrepareRemovalCandidates(t *testing.T) {
	// Setup: Create a processor with specific file extensions
	fileExtensions := []string{".rcd_sig", ".rcd", ".log"}
	p := &processor{
		id:             "test-processor",
		fileExtensions: fileExtensions,
	}

	// Mock ProcessorResult
	resp := ProcessorResult{
		Path: "/tmp/testfile.rcd_sig",
	}

	// Call prepareRemovalCandidates
	candidates := p.prepareRemovalCandidates(resp)

	// Expected removal candidates
	expectedCandidates := []string{
		"/tmp/testfile.rcd_sig",
		"/tmp/testfile.rcd",
		"/tmp/testfile.log",
	}

	// Verify the results
	assert.ElementsMatch(t, expectedCandidates, candidates)
}

func TestProcess_Remove_Success(t *testing.T) {
	// Setup: Create temporary files to simulate uploaded files
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "file1.txt")
	testFile2 := filepath.Join(tempDir, "file2.txt")
	err := os.WriteFile(testFile1, []byte("test content"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan ProcessorResult, 2)

	// Simulate successful upload results
	stored <- ProcessorResult{Path: testFile1, Error: nil}
	stored <- ProcessorResult{Path: testFile2, Error: nil}
	close(stored)

	// Create a processor instance
	fileExtensions := []string{".txt"}
	p, err := newProcessor("test-processor", nil, fileExtensions)
	assert.NoError(t, err)

	// Execute the remove function
	errors := p.remove(ctx, stored)

	// Verify no errors occurred during removal
	for err := range errors {
		assert.NoError(t, err)
	}

	// Verify files are removed
	_, exists := fsx.PathExists(testFile1)
	assert.False(t, exists)
	_, exists = fsx.PathExists(testFile2)
	assert.False(t, exists)
}

func TestProcess_Remove_Failure(t *testing.T) {
	// Setup: Create temporary files to simulate uploaded files
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "file1.txt")
	testFile2 := filepath.Join(tempDir, "file2.txt")
	err := os.WriteFile(testFile1, []byte("test content"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan ProcessorResult, 2)

	// Simulate one successful and one failed upload result
	stored <- ProcessorResult{Path: testFile1, Error: nil}
	stored <- ProcessorResult{Path: testFile2, Error: fmt.Errorf("upload failed")}
	close(stored)

	// Create a processor instance
	fileExtensions := []string{".txt"}
	p, err := newProcessor("test-processor", nil, fileExtensions)
	assert.NoError(t, err)

	// Execute the remove function
	errors := p.remove(ctx, stored)

	// Verify errors occurred for the failed upload
	var removalErrors []error
	for err := range errors {
		removalErrors = append(removalErrors, err)
	}
	assert.NotEmpty(t, removalErrors)
	assert.Equal(t, 1, len(removalErrors))

	// Verify only the successfully uploaded file is removed
	_, exists := fsx.PathExists(testFile1)
	assert.False(t, exists)
	_, exists = fsx.PathExists(testFile2)
	assert.True(t, exists)
}
