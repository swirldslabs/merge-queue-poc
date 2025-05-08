package storage

import (
	"context"
	"errors"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"sync"
)

// handler is a base struct for managing file storage operations.
//
// Fields:
//   - id: A unique identifier for the handler.
//   - storageType: The type of storage (e.g., "S3", "Local").
//   - fileExtensions: A list of file extensions to handle.
//   - pathPrefix: The prefix for the destination path.
//   - preSync: A function to validate or prepare the destination before syncing.
//   - syncFile: A function to handle the actual file synchronization.
type handler struct {
	id             string
	storageType    string
	fileExtensions []string
	pathPrefix     string
	preSync        func(ctx context.Context) error
	syncFile       func(ctx context.Context, src string, dest string) (*core.UploadInfo, error)
}

// Info returns the unique identifier of the handler.
func (h *handler) Info() string {
	return h.id
}

// Type returns the storage type of the handler.
func (h *handler) Type() string {
	return h.storageType
}

// Put uploads a file to the storage and sends the result to the provided channel.
//
// Parameters:
//   - ctx: The context for managing request deadlines and cancellations.
//   - marker: The file to be uploaded.
//   - stored: A channel to send the result of the storage operation.
func (h *handler) Put(ctx context.Context, marker core.ScannerResult, stored chan<- core.StorageResult) {
	log := logx.As().With().
		Str("marker", marker.Path).
		Str("storage_type", h.Type()).
		Str("handler", h.Info()).
		Logger()

	log.Debug().Msg("Uploading candidate file")

	var dest []string
	uploadResults, err := h.runParallel(ctx, marker.Path, h.fileExtensions)
	if err == nil {
		for _, r := range uploadResults {
			dest = append(dest, r.Dest)
		}
	}

	result := core.StorageResult{
		Error:   err,
		Src:     marker.Path,
		Dest:    dest,
		Handler: h.Info(),
		Type:    h.Type(),
	}

	if err == nil {
		log.Info().Str("marker", marker.Path).Msg(fmt.Sprintf("%s successfully handled the marker file", h.Type()))
	} else {
		log.Error().Str("marker", marker.Path).Stack().Err(err).Msg(fmt.Sprintf("%s failed to handle file", h.Type()))
	}

	select {
	case stored <- result:
	case <-ctx.Done():
		log.Warn().Msg("Context canceled while uploading to storage")
	}
}

// runParallel synchronizes multiple files in parallel based on the provided extensions.
//
// The list of candidate extensions should not be too many as it uses separate goroutines to process each file extension
// and may lead to resource exhaustion if too many goroutines are spawned. If we need to process too may (i.e. more than
// 10 extensions) for a single marker file, we should consider batching them using separate pipelines and different
// marker files. Alternatively, we need to limit the number of goroutines spawned in this method to avoid overwhelming
// the system.
//
// Parameters:
//   - ctx: The context for managing request deadlines and cancellations.
//   - markerFile: The base file to be synchronized.
//   - candidateExtensions: A list of file extensions to process.
//
// Returns:
//   - A slice of UploadInfo containing details of the uploaded files.
//   - An error if any file fails to upload.
func (h *handler) runParallel(ctx context.Context, markerFile string, candidateExtensions []string) ([]*core.UploadInfo, error) {
	if markerFile == "" || len(candidateExtensions) == 0 {
		return nil, errors.New("invalid marker file or candidate extensions")
	}

	if err := h.preSync(ctx); err != nil {
		return nil, fmt.Errorf("pre-sync validation failed: %w", err)
	}

	dir, fileName, _ := fsx.SplitFilePath(markerFile)
	var results []*core.UploadInfo
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(candidateExtensions))

	for _, ext := range candidateExtensions {
		wg.Add(1)
		go func(ext string) {
			defer wg.Done()
			src := fsx.CombineFilePath(dir, fileName, ext)
			if _, exists := fsx.PathExists(src); !exists {
				logx.As().Warn().Str("src", src).Str("storage_type", h.Type()).Msg("Source file does not exist, skipping upload")
				return
			}

			dest := fsx.CombineFilePath(h.pathPrefix, fileName, ext)

			result, err := h.syncFile(ctx, src, dest)
			if err != nil {
				errChan <- fmt.Errorf("failed to upload file %s in %s: %w", src, h.Type(), err)
				return
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(ext)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return results, nil
}
