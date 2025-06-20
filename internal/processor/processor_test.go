package processor

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/matcher"
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
	putFunc     func(ctx context.Context, item core.ScannerResult, candidates []string, stored chan<- core.StorageResult)
}

func (m *mockStorage) Info() string {
	return m.id
}

func (m *mockStorage) Type() string {
	return m.storageType
}

func (m *mockStorage) Put(ctx context.Context, item core.ScannerResult, candidates []string, stored chan<- core.StorageResult) {
	if m.putFunc != nil {
		m.putFunc(ctx, item, candidates, stored)
	} else {
		// Default behavior for testing
		stored <- core.StorageResult{
			Error:      nil,
			MarkerPath: item.Path,
			Type:       m.storageType,
			Handler:    m.id,
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

	items := make(chan core.ScannerResult, 2)
	items <- core.ScannerResult{Path: file1, Info: info1}
	items <- core.ScannerResult{Path: file2, Info: info2}
	close(items)

	ctx := context.Background()
	storages := []core.Storage{
		&mockStorage{id: "mock-storage-1", storageType: "S3"},
	}

	delay := 150 * time.Millisecond
	mc := markerCheckConfig{
		checkInterval: DefaultMarkerFileCheckInterval,
		maxAttempts:   DefaultMarkerFileCheckMaxAttempts,
		minSize:       DefaultMarkerFileCheckMinSize,
	}

	fileMatcherConfigs := []config.FileMatcherConfig{
		{
			MatcherType: matcher.FileMatcherBasic,
			Patterns:    []string{".txt"},
		},
	}
	p, err := newProcessor("test-processor", storages, fileMatcherConfigs, delay, mc)
	require.NoError(t, err)

	stored := p.upload(ctx, items)

	var results []core.ProcessorResult
	for result := range stored {
		require.NoError(t, result.Error)
		results = append(results, result)
		require.Contains(t, fileMatcherConfigs[0].Patterns, filepath.Ext(result.Path))
	}

	require.Len(t, results, 2)
}

func TestProcess_Upload_Failure(t *testing.T) {
	// Setup: Create a temporary file to simulate a scanned file
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	items := make(chan core.ScannerResult, 3)
	testFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range testFiles {
		filePath := filepath.Join(tempDir, file)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		assert.NoError(t, err)

		info, err := os.Stat(filePath)
		require.NoError(t, err)

		select {
		case items <- core.ScannerResult{Path: filePath, Info: info}:
		}
	}

	// add an invalid file, which should be ignored
	items <- core.ScannerResult{Path: filepath.Join(tempDir, "invalid.txt")}
	close(items)

	// Create a context and result channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Execute the upload function
	storages := []core.Storage{
		&mockStorage{id: "mock-storage-1", storageType: "S3",
			putFunc: func(ctx context.Context, item core.ScannerResult, candidates []string, stored chan<- core.StorageResult) {
				// Default behavior for testing
				stored <- core.StorageResult{
					Error:      fmt.Errorf("failed to upload"),
					MarkerPath: item.Path,
					Type:       "S3",
					Handler:    "mock-storage-1",
				}
			}},
	}

	delay := time.Millisecond * 150
	mc := markerCheckConfig{}
	fileMatcherConfigs := []config.FileMatcherConfig{
		{
			MatcherType: matcher.FileMatcherBasic,
			Patterns:    []string{".txt"},
		},
	}
	p, err := newProcessor("test-processor", storages, fileMatcherConfigs, delay, mc)
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
	stored := make(chan core.ProcessorResult, 2)

	// Simulate successful upload results
	stored <- core.ProcessorResult{Path: testFile1, Error: nil}
	stored <- core.ProcessorResult{Path: testFile2, Error: nil}
	close(stored)

	// Create a processor instance
	delay := time.Millisecond * 150
	mc := markerCheckConfig{}
	fileMatcherConfigs := []config.FileMatcherConfig{
		{
			MatcherType: matcher.FileMatcherBasic,
			Patterns:    []string{".txt"},
		},
	}
	p, err := newProcessor("test-processor", nil, fileMatcherConfigs, delay, mc)
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
	stored := make(chan core.ProcessorResult, 2)

	// Simulate one successful and one failed upload result
	stored <- core.ProcessorResult{Path: testFile1, Error: nil}
	stored <- core.ProcessorResult{Path: testFile2, Error: fmt.Errorf("upload failed")}
	close(stored)

	// Create a processor instance
	delay := time.Millisecond * 150
	mc := markerCheckConfig{}
	fileMatcherConfigs := []config.FileMatcherConfig{
		{
			MatcherType: matcher.FileMatcherBasic,
			Patterns:    []string{".txt"},
		},
	}
	p, err := newProcessor("test-processor", nil, fileMatcherConfigs, delay, mc)
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
	var storages []core.Storage
	fileMatcherConfigs := []config.FileMatcherConfig{
		{
			MatcherType: matcher.FileMatcherBasic,
			Patterns:    []string{".txt"},
		},
	}

	// Valid flushDelay
	pc := &config.ProcessorConfig{
		FlushDelay:         "250ms",
		FileMatcherConfigs: fileMatcherConfigs,
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
	info, err = p.waitForMarkerFileToBeReady(ctx, core.ScannerResult{Path: markerPath, Info: info})
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
	info, err = p.waitForMarkerFileToBeReady(context.Background(), core.ScannerResult{Path: markerPath, Info: info})
	require.NoError(t, err)
	require.NotNil(t, info)
	elapsed := time.Since(start)

	require.NoError(t, err, "Should not return error, just continue after maxAttempts")
	require.GreaterOrEqual(t, elapsed, 20*time.Millisecond, "Should wait for at least two intervals")
}

func TestProcessor_PrepareRemovalCandidates_DeduplicationAndOrder(t *testing.T) {
	p := &processor{}

	resp := core.ProcessorResult{
		Path: "/tmp/file1.txt",
		Result: map[string]*core.StorageResult{
			"s3": {
				Error: nil,
				UploadResults: []*core.UploadInfo{
					{Src: "/tmp/file1.txt"},
					{Src: "/tmp/file2.txt"},
				},
			},
			"gcs": {
				Error: nil,
				UploadResults: []*core.UploadInfo{
					{Src: "/tmp/file2.txt"},
					{Src: "/tmp/file3.txt"},
				},
			},
		},
	}

	got := p.prepareRemovalCandidates(resp)
	want := []string{"/tmp/file1.txt", "/tmp/file2.txt", "/tmp/file3.txt"}
	assert.Equal(t, want, got)
}

func TestProcessor_PrepareRemovalCandidates_SkipErroredStorage(t *testing.T) {
	p := &processor{}

	resp := core.ProcessorResult{
		Path: "/tmp/file1.txt",
		Result: map[string]*core.StorageResult{
			"s3": {
				Error: fmt.Errorf("upload failed"),
				UploadResults: []*core.UploadInfo{
					{Src: "/tmp/file2.txt"},
				},
			},
			"gcs": {
				Error: nil,
				UploadResults: []*core.UploadInfo{
					{Src: "/tmp/file3.txt"},
				},
			},
		},
	}

	got := p.prepareRemovalCandidates(resp)
	want := []string{"/tmp/file1.txt", "/tmp/file3.txt"}
	assert.Equal(t, want, got)
}

func TestProcessor_PrepareRemovalCandidates_EmptyResult(t *testing.T) {
	p := &processor{}

	resp := core.ProcessorResult{
		Path:   "/tmp/file1.txt",
		Result: map[string]*core.StorageResult{},
	}

	got := p.prepareRemovalCandidates(resp)
	want := []string{"/tmp/file1.txt"}
	assert.Equal(t, want, got)
}
