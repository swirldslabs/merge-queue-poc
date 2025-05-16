package config

import (
	"github.com/pkg/errors"
)

// ValidateBucketConfig validates the S3 bucket configuration.
//
// Parameters:
//   - bucketConfig: The configuration to validate.
//
// Returns:
//   - An error if any required field is missing, otherwise nil.
func ValidateBucketConfig(bucketConfig BucketConfig) error {
	if bucketConfig.AccessKey == "" {
		return errors.New("missing AccessKey in configuration")
	}
	if bucketConfig.SecretKey == "" {
		return errors.New("missing SecretKey in configuration")
	}
	if bucketConfig.Bucket == "" {
		return errors.New("missing Bucket in configuration")
	}
	if bucketConfig.Region == "" {
		return errors.New("missing Region in configuration")
	}
	if bucketConfig.Endpoint == "" {
		return errors.New("missing Endpoint in configuration")
	}
	return nil
}

// IsValidExtension ValidateLocalDirConfig validates the local directory configuration.
// for now, we only support extension begin with dot. We may support glob pattern or regex later
func IsValidExtension(ext string) bool {
	if len(ext) > 0 && ext[0] != '.' {
		return false
	}

	return true
}
