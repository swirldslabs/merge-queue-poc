package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBucketConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      BucketConfig
		expectedErr string
	}{
		{
			name: "Valid configuration",
			config: BucketConfig{
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				Bucket:    "test-bucket",
				Region:    "test-region",
				Endpoint:  "test-endpoint",
			},
			expectedErr: "",
		},
		{
			name: "Missing AccessKey",
			config: BucketConfig{
				SecretKey: "test-secret-key",
				Bucket:    "test-bucket",
				Region:    "test-region",
				Endpoint:  "test-endpoint",
			},
			expectedErr: "missing AccessKey in configuration",
		},
		{
			name: "Missing SecretKey",
			config: BucketConfig{
				AccessKey: "test-access-key",
				Bucket:    "test-bucket",
				Region:    "test-region",
				Endpoint:  "test-endpoint",
			},
			expectedErr: "missing SecretKey in configuration",
		},
		{
			name: "Missing Bucket",
			config: BucketConfig{
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				Region:    "test-region",
				Endpoint:  "test-endpoint",
			},
			expectedErr: "missing Bucket in configuration",
		},
		{
			name: "Missing Region",
			config: BucketConfig{
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				Bucket:    "test-bucket",
				Endpoint:  "test-endpoint",
			},
			expectedErr: "missing Region in configuration",
		},
		{
			name: "Missing Endpoint",
			config: BucketConfig{
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
				Bucket:    "test-bucket",
				Region:    "test-region",
			},
			expectedErr: "missing Endpoint in configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBucketConfig(tt.config)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}
