package storage

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/logx"
	"time"
)

type s3Handler struct {
	id           string
	bucketConfig config.BucketConfig
	retryConfig  config.RetryConfig
}

func (s *s3Handler) Info() string {
	return s.id
}

func (s *s3Handler) Type() string {
	return TypeS3
}

func (s *s3Handler) Put(ctx context.Context, item core.ScannerResult, stored chan<- core.StorageResult) {
	logx.As().Debug().
		Str("path", item.Path).
		Str("storage_type", s.Type()).
		Str("handler", s.Info()).
		Msg("Uploading file to S3 bucket")

	// Simulate upload delay
	time.Sleep(50 * time.Millisecond)

	dest := fmt.Sprintf("s3://%s/%s/%s", s.bucketConfig.Prefix, s.bucketConfig.Bucket, item.Path)
	//result := core.StorageResult{Src: item.Path, Dest: dest, Uploader: s.Info(), Type: s.Type(), Error: errors.New("failed to upload to s3")}
	result := core.StorageResult{Src: item.Path, Dest: dest, Uploader: s.Info(), Type: s.Type()}

	select {
	case stored <- result:
		logx.As().Debug().
			Str("path", item.Path).
			Str("storage_type", s.Type()).
			Str("handler", s.Info()).
			Msg("Uploaded file to S3 bucket")
	case <-ctx.Done():
		return
	}
}

func NewS3(id string, bucketConfig config.BucketConfig, retryConfig config.RetryConfig) (core.Storage, error) {
	return &s3Handler{id: id, bucketConfig: bucketConfig, retryConfig: retryConfig}, nil
}
