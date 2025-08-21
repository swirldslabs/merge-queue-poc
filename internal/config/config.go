package config

import (
	"fmt"
	"github.com/spf13/viper"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"golang.hedera.com/solo-cheetah/pkg/sniff"
	"os"
	"strconv"
	"strings"
)

// Config holds the global configuration for the application.
type Config struct {
	// Log contains logging-related configuration.
	Log *logx.LoggingConfig
	// Pipelines is a list of pipeline configurations.
	Pipelines []*PipelineConfig
	// Stats contains the statistics configuration.
	Profiling *sniff.ProfilingConfig
}

// PipelineConfig holds the configuration for a single pipeline.
type PipelineConfig struct {
	Enabled bool
	// Name is the name of the pipeline.
	Name string
	// Description provides a brief description of the pipeline.
	Description string
	// Scanner contains the scanner configuration.
	Scanner *ScannerConfig
	// Processor contains the processor configuration.
	Processor *ProcessorConfig
	// StopOnError indicates whether to stop the pipeline on error. We can ignore errors with the hope that the next run will succeed.
	StopOnError bool
}

// ScannerConfig holds the configuration for the scanner.
type ScannerConfig struct {
	// Directory is the directory to scan.
	Directory string
	// Pattern is the file extension pattern to match.
	Pattern string
	// Interval specifies the scan interval (e.g., "5m").
	Interval string
	// BatchSize is the number of files to process in a batch.
	BatchSize int
}

// ProcessorConfig holds the configuration for the processor.
type ProcessorConfig struct {
	// MaxProcessors is the maximum number of concurrent processors.
	MaxProcessors int
	// Retry contains the retry configuration.
	Retry *RetryConfig
	// Storage contains the storage configuration.
	Storage *StorageConfig
	// FlushDelay specifies how long to wait to allow data files to flush before starting uploads (e.g., "150ms").
	FlushDelay string
	// BackoffDelay specifies the delay between retries for failed uploads (e.g., "100ms").
	BackoffDelay string
	// MarkerCheckConfig contains the configuration for checking marker files before starting to upload.
	MarkerCheckConfig *MarkerCheckConfig
	// FileMatcherConfigs is a list of file matcher config to apply to find files to be processed for a marker file
	FileMatcherConfigs []FileMatcherConfig
}

type MarkerCheckConfig struct {
	// CheckInterval is delay between attempts to check a marker file.
	CheckInterval string
	// Maximum number of attempts to check a marker file before giving up.
	MaxAttempts int
	// Minimum size of marker files to process, in bytes. Default is 0, meaning all files are processed.
	MinSize int64
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

// LocalDirConfig holds the configuration for a local directory.
type LocalDirConfig struct {
	// Enabled indicates whether the local directory is enabled.
	Enabled bool
	// Path is the path to the local directory.
	Path string
	// Mode is the file mode for the directory.
	Mode os.FileMode
}

type FileMatcherConfig struct {
	MatcherType string
	// Patterns is a list of file patterns to process when a marker file is found.
	Patterns []string
}

var config = Config{
	Log: &logx.LoggingConfig{
		Level:          "Info",
		ConsoleLogging: true,
		FileLogging:    false,
	},
	Pipelines: []*PipelineConfig{},
	Profiling: &sniff.ProfilingConfig{
		Enabled: false,
	},
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
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	initializeNestedStructs()

	// Set values for the pipeline configuration using env vars.
	// We need this because currently viper won't override the array configuration elements using the env vars.
	err := overridePipelineConfigWithEnvVars()
	if err != nil {
		return err
	}

	return nil
}

// initializeNestedStructs ensures all nested structs are initialized.
func initializeNestedStructs() {
	if config.Profiling == nil {
		config.Profiling = &sniff.ProfilingConfig{Enabled: false}
	}

	for _, pipeline := range config.Pipelines {
		if pipeline.Scanner == nil {
			pipeline.Scanner = &ScannerConfig{}
		}
		if pipeline.Processor == nil {
			pipeline.Processor = &ProcessorConfig{}
		}

		if pipeline.Processor.MarkerCheckConfig == nil {
			pipeline.Processor.MarkerCheckConfig = &MarkerCheckConfig{
				MinSize:       0,
				MaxAttempts:   3,
				CheckInterval: "100ms",
			}
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
		if pipeline.Processor.Storage.LocalDir == nil {
			pipeline.Processor.Storage.LocalDir = &LocalDirConfig{}
		}
	}
}

// overridePipelineConfigWithEnvVars overrides pipeline configuration values with environment variables.
// We need this because currently viper won't override the array configuration elements using the env vars.
func overridePipelineConfigWithEnvVars() error {
	if config.Pipelines != nil && len(config.Pipelines) > 0 {
		for _, pipeline := range config.Pipelines {

			// set boolean fields using well-defined env vars
			booleanFields := map[string]*bool{
				"S3_ENABLED":  &pipeline.Processor.Storage.S3.Enabled,
				"S3_USE_SSL":  &pipeline.Processor.Storage.S3.UseSSL,
				"GCS_ENABLED": &pipeline.Processor.Storage.GCS.Enabled,
				"GCS_USE_SSL": &pipeline.Processor.Storage.GCS.UseSSL,
			}
			for envVar, field := range booleanFields {
				if envValue := os.Getenv(envVar); envValue != "" {
					envValueBool, err := strconv.ParseBool(envValue)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %s", envVar, envValue)
					} else {
						*field = envValueBool
					}
				}
			}

			overrideBucketConfigWithEnv(pipeline.Processor.Storage.S3)
			overrideBucketConfigWithEnv(pipeline.Processor.Storage.GCS)
		}
	}

	return nil
}

// overrideBucketConfigWithEnv overrides bucket configuration values with environment variables.
func overrideBucketConfigWithEnv(bucket *BucketConfig) {
	if bucket == nil {
		return
	}

	bucket.Bucket = overrideWithEnv(bucket.Bucket)
	bucket.Region = overrideWithEnv(bucket.Region)
	bucket.Prefix = overrideWithEnv(bucket.Prefix)
	bucket.Endpoint = overrideWithEnv(bucket.Endpoint)
	bucket.AccessKey = overrideWithEnv(bucket.AccessKey)
	bucket.SecretKey = overrideWithEnv(bucket.SecretKey)

	// convert http or https in Endpoint to use_ssl boolean
	if bucket.Endpoint != "" {
		if strings.HasPrefix(bucket.Endpoint, "https://") {
			bucket.Endpoint = strings.TrimPrefix(bucket.Endpoint, "https://")
			bucket.UseSSL = true
		} else if strings.HasPrefix(bucket.Endpoint, "http://") {
			bucket.Endpoint = strings.TrimPrefix(bucket.Endpoint, "http://")
			bucket.UseSSL = false
		}
	}
}

// overridePipelineConfigWithEnvVars overrides configuration values with environment variables.
func overrideWithEnv(value string) string {
	if envValue := os.Getenv(value); envValue != "" {
		return envValue
	}
	return value
}

// Get returns the loaded configuration.
//
// Returns:
//   - The global configuration.
func Get() Config {
	return config
}

func Set(c *Config) error {
	config = *c
	initializeNestedStructs()

	// Set values for the pipeline configuration using env vars.
	// We need this because currently viper won't override the array configuration elements using the env vars.
	err := overridePipelineConfigWithEnvVars()
	if err != nil {
		return err
	}

	return nil
}
