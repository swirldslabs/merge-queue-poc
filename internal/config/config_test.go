package config

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestInitialize(t *testing.T) {
	dataDir := "../../data"
	homeDir := path.Join(dataDir, ".cheetah", "cheetah.yaml")

	_ = os.Setenv("CHEETAH_LOG.LEVEL", "Debug") // use viper's SetEnvPrefix and automatic env var loading
	_ = os.Setenv("S3_ACCESS_KEY", "test")      // custom env var loading based on config

	err := Initialize(homeDir)
	if err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}
	assert.Equal(t, 2, len(config.Pipelines))
	assert.NotEmpty(t, config.Pipelines[0].Processor.Storage.S3.AccessKey)
	assert.Equal(t, config.Pipelines[0].Processor.Storage.S3.AccessKey, "test")
	assert.Equal(t, "Debug", config.Log.Level)

	// Test with an invalid home directory
	err = Initialize("/invalid/path")
	if err == nil {
		t.Fatal("Expected error for invalid home directory, but got none")
	}
}
