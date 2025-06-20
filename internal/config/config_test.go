package config

import (
	"github.com/stretchr/testify/require"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
	"testing"
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
    enabled: true
    description: "A test pipeline"
    scanner:
      directory: "/test/dir"
      pattern: ".txt"
      recursive: true
      interval: "5m"
      batchSize: 10
    processor:
      maxProcessors: 5
      flushDelay: "100ms"
      markerCheckConfig:
        checkInterval: 100ms
        minSize: 100
        maxAttempts: 5 
      fileMatcherConfigs:
        - matcherType: glob
          patterns: [".txt", ".log"]
        - matcherType: sidecar 
          patterns: [".gz", ".log"]
      storage:
        s3:
          enabled: false 
          bucket: "S3_BUCKET"
          endpoint: "S3_ENDPOINT"
          prefix: "S3_BUCKET_PREFIX"
          region: "S3_REGION"
          accessKey: "S3_ACCESS_KEY"
          secretKey: "S3_SECRET_KEY"
          useSSL: true
  - name: "InactiveTestPipeline"
    enabled: false 
`), 0644)
	require.NoError(t, err)

	// Unset environment variables
	_ = os.Unsetenv("S3_BUCKET")
	_ = os.Unsetenv("S3_BUCKET_PREFIX")
	_ = os.Unsetenv("CHEETAH_LOG_LEVEL")
	_ = os.Unsetenv("S3_BUCKET_PREFIX2")
	_ = os.Unsetenv("S3_ENDPOINT")
	_ = os.Unsetenv("S3_REGION")
	_ = os.Unsetenv("S3_ACCESS_KEY")
	_ = os.Unsetenv("S3_SECRET_KEY")
	_ = os.Unsetenv("S3_ENABLED")
	_ = os.Unsetenv("S3_USE_SSL")

	// Test valid initialization
	err = Initialize(configFile)
	require.NoError(t, err)
	require.Equal(t, "Debug", config.Log.Level)
	require.Equal(t, 2, len(config.Pipelines))
	require.Equal(t, "TestPipeline", config.Pipelines[0].Name)
	require.Equal(t, "/test/dir", config.Pipelines[0].Scanner.Directory)
	require.Equal(t, "S3_ACCESS_KEY", config.Pipelines[0].Processor.Storage.S3.AccessKey)
	require.Equal(t, "100ms", config.Pipelines[0].Processor.FlushDelay)
	require.Equal(t, "100ms", config.Pipelines[0].Processor.MarkerCheckConfig.CheckInterval)
	require.Equal(t, int64(100), config.Pipelines[0].Processor.MarkerCheckConfig.MinSize)
	require.Equal(t, 5, config.Pipelines[0].Processor.MarkerCheckConfig.MaxAttempts)
	require.Equal(t, true, config.Pipelines[0].Enabled)
	require.Equal(t, false, config.Pipelines[1].Enabled)
	require.ElementsMatch(t, []string{".txt", ".log"}, config.Pipelines[0].Processor.FileMatcherConfigs[0].Patterns)

	_ = os.Setenv("S3_BUCKET", "bucket")
	_ = os.Setenv("S3_BUCKET_PREFIX", "bucket-prefix")
	_ = os.Setenv("CHEETAH_LOG_LEVEL", "Warn")
	_ = os.Setenv("S3_BUCKET_PREFIX2", "bucket-prefix")
	_ = os.Setenv("S3_ENDPOINT", "localhost:9000")
	_ = os.Setenv("S3_REGION", "region")
	_ = os.Setenv("S3_ACCESS_KEY", "test")
	_ = os.Setenv("S3_SECRET_KEY", "secret")
	_ = os.Setenv("S3_ENABLED", "true")
	_ = os.Setenv("S3_USE_SSL", "false")
	err = Initialize(configFile)
	require.NoError(t, err)
	require.Equal(t, "Warn", config.Log.Level)
	require.Equal(t, true, config.Pipelines[0].Processor.Storage.S3.Enabled)
	require.Equal(t, false, config.Pipelines[0].Processor.Storage.S3.UseSSL)
	require.Equal(t, "bucket", config.Pipelines[0].Processor.Storage.S3.Bucket)
	require.Equal(t, "bucket-prefix", config.Pipelines[0].Processor.Storage.S3.Prefix)
	require.Equal(t, "region", config.Pipelines[0].Processor.Storage.S3.Region)
	require.Equal(t, "localhost:9000", config.Pipelines[0].Processor.Storage.S3.Endpoint)
	require.Equal(t, "test", config.Pipelines[0].Processor.Storage.S3.AccessKey)
	require.Equal(t, "secret", config.Pipelines[0].Processor.Storage.S3.SecretKey)

	// Test invalid initialization
	err = Initialize("/invalid/path")
	require.Error(t, err)
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
	require.Equal(t, "Info", result.Log.Level)
	require.Equal(t, 1, len(result.Pipelines))
	require.Equal(t, "MockPipeline", result.Pipelines[0].Name)
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
	require.Equal(t, "Error", config.Log.Level)
	require.Equal(t, 1, len(config.Pipelines))
	require.Equal(t, "NewPipeline", config.Pipelines[0].Name)
}
