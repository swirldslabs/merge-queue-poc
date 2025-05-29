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

func Test_handler_computeDestinationPath(t *testing.T) {
	type fields struct {
		id             string
		storageType    string
		fileExtensions []string
		rootDir        string
		pathPrefix     string
		preSync        func(ctx context.Context) error
		syncFile       func(ctx context.Context, src string, dest string) (*core.UploadInfo, error)
	}
	type args struct {
		srcDir   string
		fileName string
		ext      string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "Test with sub directory",
			fields: fields{
				id:             "test-handler",
				storageType:    "local",
				fileExtensions: []string{".txt", ".jpg"},
				rootDir:        "/root",
				pathPrefix:     "uploads",
			},
			args: args{
				srcDir:   "/root/recordStream/record0.0.10",
				fileName: "file",
				ext:      ".txt",
			},
			want: "uploads/recordStream/record0.0.10/file.txt",
		},
		{
			name: "Test without sub directory",
			fields: fields{
				id:             "test-handler",
				storageType:    "local",
				fileExtensions: []string{".txt", ".jpg"},
				rootDir:        "/root",
				pathPrefix:     "uploads",
			},
			args: args{
				srcDir:   "/root",
				fileName: "file",
				ext:      ".txt",
			},
			want: "uploads/file.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				id:             tt.fields.id,
				storageType:    tt.fields.storageType,
				fileExtensions: tt.fields.fileExtensions,
				rootDir:        tt.fields.rootDir,
				pathPrefix:     tt.fields.pathPrefix,
				preSync:        tt.fields.preSync,
				syncFile:       tt.fields.syncFile,
			}
			if got := h.computeDestinationPath(tt.args.srcDir, tt.args.fileName, tt.args.ext); got != tt.want {
				t.Errorf("computeDestinationPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
		id:             "test-handler",
		storageType:    "local",
		fileExtensions: []string{".mf", ".data"},
		rootDir:        tempDir,
		pathPrefix:     "uploads",
		syncFile:       mockSyncFile,
	}

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan core.StorageResult, 1)

	// Execute the Put function
	go func() {
		defer close(stored)
		h.Put(ctx, core.ScannerResult{Path: markerFile}, stored)
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
		id:             "test-handler",
		storageType:    "local",
		fileExtensions: []string{".mf", ".data"},
		rootDir:        tempDir,
		pathPrefix:     "uploads",
		syncFile:       mockSyncFile,
	}

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan core.StorageResult, 1)

	// Execute the Put function
	go func() {
		defer close(stored)
		h.Put(ctx, core.ScannerResult{Path: markerFile}, stored)
	}()

	// Verify the result
	for result := range stored {
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "candidate file is missing")
		assert.Contains(t, result.Src, "file.mf")
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
		id:             "test-handler",
		storageType:    "local",
		fileExtensions: []string{".mf", ".data"},
		rootDir:        tempDir,
		pathPrefix:     "uploads",
		syncFile:       mockSyncFile,
	}

	// Create a context and result channel
	ctx := context.Background()
	stored := make(chan core.StorageResult, 1)

	// Execute the Put function
	go func() {
		defer close(stored)
		h.Put(ctx, core.ScannerResult{Path: markerFile}, stored)
	}()

	// Verify the result
	for result := range stored {
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "failed to upload file")
		assert.Contains(t, result.Src, "file.mf")
	}
}
