package config

import (
	"fmt"
	"github.com/spf13/viper"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
)

// Config holds the global configuration for the application.
type Config struct {
	// Log contains logging-related configuration.
	Log *logx.LoggingConfig
	// Pipelines is a list of pipeline configurations.
	Pipelines []*PipelineConfig
}

// PipelineConfig holds the configuration for a single pipeline.
type PipelineConfig struct {
	// Name is the name of the pipeline.
	Name string
	// Description provides a brief description of the pipeline.
	Description string
	// Scanner contains the scanner configuration.
	Scanner *ScannerConfig
	// Processor contains the processor configuration.
	Processor *ProcessorConfig
}

// ScannerConfig holds the configuration for the scanner.
type ScannerConfig struct {
	// Path is the directory to scan.
	Path string
	// Pattern is the file pattern to match.
	Pattern string
	// Recursive enables recursive scanning of directories.
	Recursive bool
	// Interval specifies the scan interval (e.g., "5m").
	Interval string
}

// ProcessorConfig holds the configuration for the processor.
type ProcessorConfig struct {
	// MaxProcessors is the maximum number of concurrent processors.
	MaxProcessors int
	// Retry contains the retry configuration.
	Retry *RetryConfig
	// Storage contains the storage configuration.
	Storage *StorageConfig
	// FileExtensions is a list of file extensions to process.
	FileExtensions []string
}

// RetryConfig holds the configuration for retrying failed operations.
type RetryConfig struct {
	// Limit is the maximum number of retry attempts.
	Limit int
}

// StorageConfig holds the configuration for storage backends.
type StorageConfig struct {
	// S3 contains the S3 bucket configuration.
	S3 *BucketConfig
	// GCS contains the Google Cloud Storage bucket configuration.
	GCS *BucketConfig
	// RemoteHost contains the remote host configuration.
	RemoteHost *RemoteHostConfig
	// LocalDir contains the local directory configuration.
	LocalDir *LocalDirConfig
}

// BucketConfig holds the configuration for an S3 or GCS bucket.
type BucketConfig struct {
	// Enabled indicates whether the bucket is enabled.
	Enabled bool
	// Bucket is the name of the bucket.
	Bucket string
	// Region is the region of the bucket.
	Region string
	// Prefix is the prefix for objects in the bucket.
	Prefix string
	// Endpoint is the endpoint for the bucket.
	Endpoint string
	// AccessKey is the access key for the bucket.
	AccessKey string
	// SecretKey is the secret key for the bucket.
	SecretKey string
	// UseSSL enables SSL for the bucket connection.
	UseSSL bool
}

// RemoteHostConfig holds the configuration for a remote host.
type RemoteHostConfig struct {
	// Enabled indicates whether the remote host is enabled.
	Enabled bool
	// Host is the hostname or IP address of the remote host.
	Host string
	// Port is the port number of the remote host.
	Port int
	// Path is the path on the remote host.
	Path string
	// Username is the username for authentication.
	Username string
	// Password is the password for authentication.
	Password string
}

// LocalDirConfig holds the configuration for a local directory.
type LocalDirConfig struct {
	// Enabled indicates whether the local directory is enabled.
	Enabled bool
	// Path is the path to the local directory.
	Path string
	// Mode is the file mode for the directory.
	Mode os.FileMode
}

var config = Config{
	Log: &logx.LoggingConfig{
		Level:          "Info",
		ConsoleLogging: true,
		FileLogging:    false,
	},
	Pipelines: []*PipelineConfig{},
}

// Initialize loads the configuration from the specified file.
//
// Parameters:
//   - path: The path to the configuration file.
//
// Returns:
//   - An error if the configuration cannot be loaded.
func Initialize(path string) error {
	viper.Reset()
	viper.SetConfigFile(path)
	viper.SetEnvPrefix("cheetah")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	initializeNestedStructs()
	overrideWithEnvVars()

	return nil
}

// initializeNestedStructs ensures all nested structs are initialized.
func initializeNestedStructs() {
	for _, pipeline := range config.Pipelines {
		if pipeline.Scanner == nil {
			pipeline.Scanner = &ScannerConfig{}
		}
		if pipeline.Processor == nil {
			pipeline.Processor = &ProcessorConfig{}
		}
		if pipeline.Processor.Storage == nil {
			pipeline.Processor.Storage = &StorageConfig{}
		}
		if pipeline.Processor.Storage.S3 == nil {
			pipeline.Processor.Storage.S3 = &BucketConfig{}
		}
		if pipeline.Processor.Storage.GCS == nil {
			pipeline.Processor.Storage.GCS = &BucketConfig{}
		}
		if pipeline.Processor.Storage.RemoteHost == nil {
			pipeline.Processor.Storage.RemoteHost = &RemoteHostConfig{}
		}
		if pipeline.Processor.Storage.LocalDir == nil {
			pipeline.Processor.Storage.LocalDir = &LocalDirConfig{}
		}
	}
}

// overrideWithEnvVars overrides sensitive fields with environment variables if set.
func overrideWithEnvVars() {
	for _, pipeline := range config.Pipelines {
		if pipeline.Processor.Storage.S3.AccessKey != "" {
			pipeline.Processor.Storage.S3.AccessKey = os.Getenv(pipeline.Processor.Storage.S3.AccessKey)
		}
		if pipeline.Processor.Storage.S3.SecretKey != "" {
			pipeline.Processor.Storage.S3.SecretKey = os.Getenv(pipeline.Processor.Storage.S3.SecretKey)
		}
		if pipeline.Processor.Storage.GCS.AccessKey != "" {
			pipeline.Processor.Storage.GCS.AccessKey = os.Getenv(pipeline.Processor.Storage.GCS.AccessKey)
		}
		if pipeline.Processor.Storage.GCS.SecretKey != "" {
			pipeline.Processor.Storage.GCS.SecretKey = os.Getenv(pipeline.Processor.Storage.GCS.SecretKey)
		}
	}
}

// Get returns the loaded configuration.
//
// Returns:
//   - The global configuration.
func Get() Config {
	return config
}
