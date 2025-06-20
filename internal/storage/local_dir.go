package storage

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
)

type localDirectoryHandler struct {
	*handler
	dirConfig   config.LocalDirConfig
	retryConfig config.RetryConfig
}

// ensureDirExists checks if the local directory exists. If it doesn't, it creates the directory.
func (d *localDirectoryHandler) ensureDirExists(ctx context.Context) error {
	if _, exists := fsx.PathExists(d.dirConfig.Path); exists {
		logx.As().Trace().
			Str("storage_type", d.Type()).
			Str("path", d.dirConfig.Path).
			Msg("Directory already exists and was previously checked")
		return nil
	}

	logx.As().Debug().
		Str("storage_type", d.Type()).
		Str("path", d.dirConfig.Path).
		Msg("Directory does not exist, creating it")

	if err := os.MkdirAll(d.dirConfig.Path, d.dirConfig.Mode); err != nil {
		logx.As().Error().
			Str("storage_type", d.Type()).
			Str("path", d.dirConfig.Path).
			Err(err).
			Msg("Failed to create directory")
		return fmt.Errorf("failed to create directory: %w", err)
	}

	logx.As().Debug().
		Str("storage_type", d.Type()).
		Str("path", d.dirConfig.Path).
		Msg("Directory created successfully")
	return nil
}

// syncWithDir copies a file to the local directory. It skips copying if the file already exists with the same checksum.
func (d *localDirectoryHandler) syncWithDir(ctx context.Context, src string, dest string) (*core.UploadInfo, error) {
	var err error
	var localChecksum, remoteChecksum string

	// prepend the root directory to the destination path
	dest = filepath.Join(d.dirConfig.Path, dest)

	logx.As().Debug().
		Str("src", src).
		Str("dest", dest).
		Str("id", d.Info()).
		Msg("Starting file synchronization with local directory")

	info, exists := fsx.PathExists(src)
	if !exists {
		logx.As().Error().
			Str("src", src).
			Msg("Source file does not exist")
		return nil, fmt.Errorf("source file does not exist: %w", err)
	}

	localChecksum, err = fsx.FileMD5(src)
	if err != nil {
		logx.As().Error().
			Str("src", src).
			Err(err).
			Msg("Failed to calculate local file checksum")
		return nil, fmt.Errorf("failed to calculate local checksum: %w", err)
	}

	logx.As().Debug().Str("src", src).Str("dest", dest).
		Str("local_checksum", localChecksum).
		Str("id", d.Info()).
		Msg("Computed local file checksum")

	if destInfo, exists := fsx.PathExists(dest); exists {
		remoteChecksum, err = fsx.FileMD5(dest)
		if err != nil {
			logx.As().Error().
				Str("dest", dest).
				Err(err).
				Msg("Failed to calculate remote file checksum")
			return nil, fmt.Errorf("failed to calculate remote checksum: %w", err)
		}

		logx.As().Debug().
			Str("src", src).
			Str("dest", dest).
			Str("local_checksum", localChecksum).
			Str("remote_checksum", remoteChecksum).
			Str("id", d.Info()).
			Msg("Computed remote file checksum")

		if localChecksum == remoteChecksum {
			logx.As().Info().
				Str("src", src).
				Str("dest", dest).
				Str("local_checksum", localChecksum).
				Str("remote_checksum", remoteChecksum).
				Str("storage_type", d.Type()).
				Str("id", d.Info()).
				Msg("File already exists in the local directory, skipping copy")
			return d.prepareUploadInfo(src, dest, remoteChecksum, destInfo)
		}
	}

	destDir := filepath.Dir(dest)
	if _, exists := fsx.PathExists(destDir); !exists {
		logx.As().Debug().
			Str("storage_type", d.Type()).
			Str("path", destDir).
			Str("id", d.Info()).
			Msg("Destination directory does not exist, creating it")

		if err := os.MkdirAll(destDir, d.dirConfig.Mode); err != nil {
			logx.As().Error().
				Str("storage_type", d.Type()).
				Str("path", destDir).
				Str("id", d.Info()).
				Err(err).
				Msg("Failed to create destination directory")
			return nil, fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	logx.As().Debug().
		Str("src", src).
		Str("dest", dest).
		Str("checksum", localChecksum).
		Str("storage_type", d.Type()).
		Str("id", d.Info()).
		Msg("Copying file to the local directory")

	if err = fsx.Copy(src, dest, d.dirConfig.Mode); err != nil {
		logx.As().Error().
			Str("src", src).
			Str("dest", dest).
			Err(err).
			Msg("Failed to copy file to the local directory")
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	logx.As().Info().
		Str("src", src).
		Str("dest", dest).
		Str("checksum", localChecksum).
		Str("storage_type", d.Type()).
		Str("size", fmt.Sprintf("%d bytes", info.Size())).
		Str("id", d.Info()).
		Msg("File copied successfully to the local directory")

	return d.prepareUploadInfo(src, dest, localChecksum, info)
}

// prepareUploadInfo prepares the upload information for a file.
func (d *localDirectoryHandler) prepareUploadInfo(src string, dest string, checksum string, info os.FileInfo) (*core.UploadInfo, error) {
	return &core.UploadInfo{
		Src:          src,
		Dest:         dest,
		ChecksumType: "md5",
		Checksum:     checksum,
		Size:         info.Size(),
		LastModified: info.ModTime(),
	}, nil
}

// NewLocalDir creates a new local directory storage handler.
func NewLocalDir(id string, config config.LocalDirConfig, retryConfig config.RetryConfig, rootDir string) (core.Storage, error) {
	return newLocalDir(id, config, retryConfig, rootDir)
}

func newLocalDir(id string, config config.LocalDirConfig, retryConfig config.RetryConfig, rootDir string) (*localDirectoryHandler, error) {
	l := &localDirectoryHandler{
		handler: &handler{
			id:          id,
			storageType: TypeLocalDir,
			rootDir:     rootDir,
		},
		dirConfig:   config,
		retryConfig: retryConfig,
	}

	// Initialize the handler functions
	l.handler.preSync = l.ensureDirExists
	l.handler.syncFile = l.syncWithDir

	logx.As().Trace().
		Str("id", l.Info()).
		Str("storage_type", TypeLocalDir).
		Str("path", config.Path).
		Msg("Local directory storage handler created successfully")

	return l, nil
}
