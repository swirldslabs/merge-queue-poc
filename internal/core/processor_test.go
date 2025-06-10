package core

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("test content"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("test content"), 0644))

	info1, err := os.Stat(file1)
	require.NoError(t, err)
	info2, err := os.Stat(file2)
	require.NoError(t, err)

	items := make(chan ScannerResult, 2)
	items <- ScannerResult{Path: file1, Info: info1}
	items <- ScannerResult{Path: file2, Info: info2}
	close(items)

	ctx := context.Background()
	storages := []Storage{
		&mockStorage{id: "mock-storage-1", storageType: "S3"},
	}
	fileExtensions := []string{".txt"}
	delay := 150 * time.Millisecond
	mc := markerCheckConfig{
		checkInterval: DefaultMarkerFileCheckInterval,
		maxAttempts:   DefaultMarkerFileCheckMaxAttempts,
		minSize:       DefaultMarkerFileCheckMinSize,
	}
	p, err := newProcessor("test-processor", storages, fileExtensions, delay, mc)
	require.NoError(t, err)

	stored := p.upload(ctx, items)

	var results []ProcessorResult
	for result := range stored {
		require.NoError(t, result.Error)
		results = append(results, result)
		require.Contains(t, fileExtensions, filepath.Ext(result.Path))
	}

	require.Len(t, results, 2)
}

func TestProcess_Upload_Failure(t *testing.T) {
	// Setup: Create a temporary file to simulate a scanned file
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	items := make(chan ScannerResult, 3)
	testFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range testFiles {
		filePath := filepath.Join(tempDir, file)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		assert.NoError(t, err)

		info, err := os.Stat(filePath)
		require.NoError(t, err)

		select {
		case items <- ScannerResult{Path: filePath, Info: info}:
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
	delay := time.Millisecond * 150
	mc := markerCheckConfig{}
	p, err := newProcessor("test-processor", storages, fileExtensions, delay, mc)
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
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

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
	delay := time.Millisecond * 150
	mc := markerCheckConfig{}
	p, err := newProcessor("test-processor", nil, fileExtensions, delay, mc)
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
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

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
	delay := time.Millisecond * 150
	mc := markerCheckConfig{}
	p, err := newProcessor("test-processor", nil, fileExtensions, delay, mc)
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

func TestNewProcessor_DelayParsing(t *testing.T) {
	var storages []Storage
	fileExtensions := []string{".txt"}

	// Valid flushDelay
	pc := &config.ProcessorConfig{
		FlushDelay:     "250ms",
		FileExtensions: fileExtensions,
	}

	p, err := NewProcessor("test", storages, pc)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, 250*time.Millisecond, p.(*processor).flushDelay)

	// Default flushDelay (0)
	pc.FlushDelay = "0ms"
	p, err = NewProcessor("test", storages, pc)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, time.Millisecond*0, p.(*processor).flushDelay)

	// Default flushDelay (empty string)
	pc.FlushDelay = ""
	p, err = NewProcessor("test", storages, pc)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, 150*time.Millisecond, p.(*processor).flushDelay)

	// Invalid flushDelay
	pc.FlushDelay = "notaduration"
	p, err = NewProcessor("test", storages, pc)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestIntegration_WaitForMarkerFileToBeReady(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	markerPath := filepath.Join(tempDir, "marker.ready")

	// Create a small marker file
	err := os.WriteFile(markerPath, []byte("abc"), 0644)
	require.NoError(t, err)

	// Processor with small minSize and short interval for fast test
	p := &processor{
		markerCheckConfig: markerCheckConfig{
			minSize:       10,
			maxAttempts:   10,
			checkInterval: 20 * time.Millisecond,
		},
	}

	info, err := os.Stat(markerPath)
	require.NoError(t, err)

	// Grow the file in the background after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(markerPath, []byte("abcdefghijk"), 0644)
	}()

	ctx := context.Background()
	info, err = p.waitForMarkerFileToBeReady(ctx, ScannerResult{Path: markerPath, Info: info})
	require.NoError(t, err)
	require.NotNil(t, info)

	// File should be at least minSize
	finalInfo, err := os.Stat(markerPath)
	require.NoError(t, err)
	require.GreaterOrEqual(t, finalInfo.Size(), int64(10))
}

func TestProcessor_WaitForMarkerFileToBeReady_MaxAttempts(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	markerPath := filepath.Join(tempDir, "marker.txt")

	// Create a marker file smaller than minSize
	err := os.WriteFile(markerPath, []byte("abc"), 0644)
	require.NoError(t, err)

	p := &processor{
		markerCheckConfig: markerCheckConfig{
			minSize:       10, // Require at least 10 bytes
			maxAttempts:   2,  // Only allow 2 attempts
			checkInterval: 10 * time.Millisecond,
		},
	}

	info, err := os.Stat(markerPath)
	require.NoError(t, err)

	start := time.Now()
	info, err = p.waitForMarkerFileToBeReady(context.Background(), ScannerResult{Path: markerPath, Info: info})
	require.NoError(t, err)
	require.NotNil(t, info)
	elapsed := time.Since(start)

	require.NoError(t, err, "Should not return error, just continue after maxAttempts")
	require.GreaterOrEqual(t, elapsed, 20*time.Millisecond, "Should wait for at least two intervals")
}
