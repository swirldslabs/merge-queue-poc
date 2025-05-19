package config

import (
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	// Create a temporary configuration file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(`
log:
  level: "Debug"
pipelines:
  - name: "TestPipeline"
    description: "A test pipeline"
    scanner:
      directory: "/test/dir"
      pattern: ".txt"
      recursive: true
      interval: "5m"
      batchSize: 10
    processor:
      maxProcessors: 5
      fileExtensions: [".txt", ".data"]
      storage:
        s3:
          enabled: true
          bucket: "test-bucket"
          region: "us-east-1"
          accessKey: "test-access-key"
          secretKey: "test-secret-key"
`), 0644)
	assert.NoError(t, err)

	// Test valid initialization
	err = Initialize(configFile)
	assert.NoError(t, err)
	assert.Equal(t, "Debug", config.Log.Level)
	assert.Equal(t, 1, len(config.Pipelines))
	assert.Equal(t, "TestPipeline", config.Pipelines[0].Name)
	assert.Equal(t, "/test/dir", config.Pipelines[0].Scanner.Directory)

	// Test invalid initialization
	err = Initialize("/invalid/path")
	assert.Error(t, err)
}

func TestGet(t *testing.T) {
	// Set a mock configuration
	mockConfig := Config{
		Log: &logx.LoggingConfig{
			Level: "Info",
		},
		Pipelines: []*PipelineConfig{
			{
				Name: "MockPipeline",
			},
		},
	}
	Set(&mockConfig)

	// Test Get function
	result := Get()
	assert.Equal(t, "Info", result.Log.Level)
	assert.Equal(t, 1, len(result.Pipelines))
	assert.Equal(t, "MockPipeline", result.Pipelines[0].Name)
}

func TestSet(t *testing.T) {
	// Create a new configuration
	newConfig := Config{
		Log: &logx.LoggingConfig{
			Level: "Error",
		},
		Pipelines: []*PipelineConfig{
			{
				Name: "NewPipeline",
			},
		},
	}

	// Set the new configuration
	Set(&newConfig)

	// Verify the configuration was updated
	assert.Equal(t, "Error", config.Log.Level)
	assert.Equal(t, 1, len(config.Pipelines))
	assert.Equal(t, "NewPipeline", config.Pipelines[0].Name)
}
