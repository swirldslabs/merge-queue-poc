package storage

import (
	"context"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/fsx"
	"golang.hedera.com/solo-cheetah/internal/logx"
	"os"
	"path/filepath"
)

type localDirectoryHandler struct {
	id          string
	config      config.LocalDirConfig
	retryConfig config.RetryConfig
}

func (d *localDirectoryHandler) Type() string {
	return TypeLocalDir
}

func (d *localDirectoryHandler) Info() string {
	return d.id
}

func (d *localDirectoryHandler) Put(ctx context.Context, item core.ScannerResult, stored chan<- core.StorageResult) {
	dest := filepath.Join(d.config.Path, filepath.Base(item.Path))
	logx.As().Debug().
		Str("path", item.Path).
		Str("dest", dest).
		Str("storage_type", d.Type()).
		Str("handler", d.Info()).
		Msg("Copying file to destination directory")

	var err error
	if !fsx.PathExists(d.config.Path) {
		err = os.MkdirAll(d.config.Path, d.config.Mode)
	}

	if err == nil {
		err = fsx.Copy(item.Path, dest, d.config.Mode)
	}

	result := core.StorageResult{
		Error:    err,
		Src:      item.Path,
		Dest:     dest,
		Uploader: d.Info(),
		Type:     d.Type(),
	}

	select {
	case stored <- result:
		logx.As().Debug().
			Str("path", item.Path).
			Str("dest", dest).
			Str("storage_type", d.Type()).
			Str("handler", d.Info()).
			Msg("Copied file to destination directory")
	case <-ctx.Done():
		return
	}
}

func NewLocalDir(id string, config config.LocalDirConfig, retryConfig config.RetryConfig) (core.Storage, error) {
	return &localDirectoryHandler{id: id, config: config, retryConfig: retryConfig}, nil
}
