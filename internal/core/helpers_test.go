package core

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsValidExtension(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{
			name:     "Valid extension with dot",
			ext:      ".txt",
			expected: true,
		},
		{
			name:     "Invalid extension without dot",
			ext:      "txt",
			expected: false,
		},
		{
			name:     "Invalid extension with glob pattern",
			ext:      "*.txt",
			expected: false,
		},
		{
			name:     "Empty extension",
			ext:      "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFileExtension(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeDestinationBucketPath(t *testing.T) {
	tests := []struct {
		name             string
		rootDir          string
		srcFile          string
		bucketPathPrefix string
		expected         string
	}{
		{
			name:             "file in root directory",
			rootDir:          "/data",
			srcFile:          "/data/file.txt",
			bucketPathPrefix: "bucket",
			expected:         "bucket/file.txt",
		},
		{
			name:             "file in subdirectory",
			rootDir:          "/data",
			srcFile:          "/data/subdir/file.txt",
			bucketPathPrefix: "bucket",
			expected:         "bucket/subdir/file.txt",
		},
		{
			name:             "empty bucket prefix",
			rootDir:          "/data",
			srcFile:          "/data/file.txt",
			bucketPathPrefix: "",
			expected:         "file.txt",
		},
		{
			name:             "rootDir is not prefix of srcFile",
			rootDir:          "/other",
			srcFile:          "/data/file.txt",
			bucketPathPrefix: "bucket",
			expected:         "bucket/data/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeDestinationBucketPath(tt.rootDir, tt.srcFile, tt.bucketPathPrefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}
