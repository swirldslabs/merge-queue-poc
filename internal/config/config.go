package config

import (
	"github.com/spf13/viper"
	"os"
)

var config = Config{
	Log: &LoggingConfig{
		Level:          "Info",
		ConsoleLogging: true,
		FileLogging:    false,
	},
	Pipelines: []*PipelineConfig{},
}

type Config struct {
	Log       *LoggingConfig
	Pipelines []*PipelineConfig
}

type LoggingConfig struct {
	// Level is the log level to use
	Level string
	// Enable console logging
	ConsoleLogging bool
	// FileLoggingEnabled makes the framework log to a file
	// the fields below can be skipped if this value is false!
	FileLogging bool
	// Directory to log to when file logging is enabled
	Directory string
	// Filename is the name of the logfile which will be placed inside the directory
	Filename string
	// MaxSize the max size in MB of the logfile before it's rolled
	MaxSize int
	// MaxBackups the max number of rolled files to keep
	MaxBackups int
	// MaxAge the max age in days to keep a logfile
	MaxAge int
	// Compress makes the log framework compress the rolled files
	Compress bool
}

type PipelineConfig struct {
	Name        string
	Description string
	Scanner     *ScannerConfig
	Processor   *ProcessorConfig
}

type ScannerConfig struct {
	Path      string
	Pattern   string
	Recursive bool
	Interval  string
}

type ProcessorConfig struct {
	MaxProcessors int
	Retry         *RetryConfig
	Storage       *StorageConfig
}

type RetryConfig struct {
	Limit    int
	Interval string
	Backoff  string
}

type StorageConfig struct {
	S3         *BucketConfig
	GCS        *BucketConfig
	RemoteHost *RemoteHostConfig
	LocalDir   *LocalDirConfig
}

type BucketConfig struct {
	Enabled   bool
	Bucket    string
	Region    string
	Prefix    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

type RemoteHostConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Path     string
	Username string
	Password string
}

type LocalDirConfig struct {
	Enabled bool
	Path    string
	Mode    os.FileMode
}

func Initialize(path string) error {
	viper.Reset()
	viper.SetConfigFile(path)
	viper.SetEnvPrefix("cheetah")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	err := viper.Unmarshal(&config)
	if err != nil {
		return err
	}

	// Ensure all nested structs are initialized
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

	// Override AccessKey and SecretKey with environment variables if set
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

	return nil
}

func Get() Config {
	return config
}
