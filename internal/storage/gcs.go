package storage

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/logx"
	"time"
)

type gcsHandler struct {
	id           string
	bucketConfig config.BucketConfig
	retryConfig  config.RetryConfig
}

func (g *gcsHandler) Type() string {
	return TypeGCS
}

func (g *gcsHandler) Info() string {
	return g.id
}

func (g *gcsHandler) Put(ctx context.Context, item core.ScannerResult, stored chan<- core.StorageResult) {
	logx.As().Debug().
		Str("path", item.Path).
		Str("storage_type", g.Type()).
		Str("handler", g.Info()).
		Msg("Uploading file to GCS bucket")

	// Simulate upload delay
	time.Sleep(50 * time.Millisecond)

	dest := fmt.Sprintf("gcs://%s/%s/%s", g.bucketConfig.Prefix, g.bucketConfig.Bucket, item.Path)
	result := core.StorageResult{Src: item.Path, Dest: dest, Uploader: g.Info(), Type: g.Type()}

	select {
	case stored <- result:
		logx.As().Debug().
			Str("path", item.Path).
			Str("storage_type", g.Type()).
			Str("handler", g.Info()).
			Msg("Uploaded file to GCS bucket")
	case <-ctx.Done():
		return
	}
}

func NewGCS(id string, bucketConfig config.BucketConfig, retryConfig config.RetryConfig) (core.Storage, error) {
	return &gcsHandler{id: id, bucketConfig: bucketConfig, retryConfig: retryConfig}, nil
}
