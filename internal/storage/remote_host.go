package storage

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/logx"
	"time"
)

type remoteHostHandler struct {
	id          string
	config      config.RemoteHostConfig
	retryConfig config.RetryConfig
}

func (r *remoteHostHandler) Type() string {
	return TypeRemoteHost
}

func (r *remoteHostHandler) Info() string {
	return r.id
}

func (r *remoteHostHandler) Put(ctx context.Context, item core.ScannerResult, stored chan<- core.StorageResult) {
	logx.As().Debug().
		Str("path", item.Path).
		Str("storage_type", r.Type()).
		Str("handler", r.Info()).
		Msg("Uploading file to remote host")

	// Simulate upload delay
	time.Sleep(50 * time.Millisecond)

	dest := fmt.Sprintf("%s@%s:%s", r.config.Username, r.config.Host, item.Path)
	result := core.StorageResult{Src: item.Path, Dest: dest, Uploader: r.Info(), Type: r.Type()}

	select {
	case stored <- result:
		logx.As().Debug().
			Str("path", item.Path).
			Str("storage_type", r.Type()).
			Str("handler", r.Info()).
			Msg("Uploaded file to remote host")
	case <-ctx.Done():
		return
	}
}

func NewRemoteHost(id string, config config.RemoteHostConfig, retryConfig config.RetryConfig) (core.Storage, error) {
	return &remoteHostHandler{id: id, config: config, retryConfig: retryConfig}, nil
}
