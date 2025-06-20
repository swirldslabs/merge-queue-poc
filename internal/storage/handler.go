package storage

import (
	"context"
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
//   - pathPrefix: The prefix for the destination path.
//   - preSync: A function to validate or prepare the destination before syncing.
//   - syncFile: A function to handle the actual file synchronization.
type handler struct {
	id          string
	storageType string
	rootDir     string
	pathPrefix  string
	preSync     func(ctx context.Context) error
	syncFile    func(ctx context.Context, src string, dest string) (*core.UploadInfo, error)
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
func (h *handler) Put(ctx context.Context, marker core.ScannerResult, candidates []string, stored chan<- core.StorageResult) {
	log := logx.As().With().
		Str("marker", marker.Path).
		Str("trace_id", marker.TraceId).
		Str("storage_type", h.Type()).
		Str("id", h.Info()).
		Logger()

	log.Debug().
		Str("candidates", fmt.Sprintf("%v", candidates)).
		Msg("Identified candidate files to be uploaded")

	uploadResults, err := h.runParallel(ctx, candidates)
	result := core.StorageResult{
		Error:         err,
		MarkerPath:    marker.Path,
		UploadResults: uploadResults,
		Handler:       h.Info(),
		Type:          h.Type(),
	}

	if err == nil {
		log.Trace().Msg(fmt.Sprintf("%s successfully handled the marker file", h.Type()))
	} else {
		log.Error().Stack().Err(err).Msg(fmt.Sprintf("%s failed to handle file", h.Type()))
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
//
// Returns:
//   - A slice of UploadInfo containing details of the uploaded files.
//   - An error if any file fails to upload.
func (h *handler) runParallel(ctx context.Context, candidates []string) ([]*core.UploadInfo, error) {
	if h.preSync != nil {
		if err := h.preSync(ctx); err != nil {
			return nil, fmt.Errorf("pre-sync validation failed: %w", err)
		}
	}

	var results []*core.UploadInfo
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(candidates))

	for _, candidate := range candidates {
		if _, exists := fsx.PathExists(candidate); !exists {
			errChan <- fmt.Errorf("candidate file is missing, failed to upload file %s in %s", candidate, h.Type())
		}

		bucketDest := core.ComputeDestinationBucketPath(h.rootDir, candidate, h.pathPrefix)

		wg.Add(1)
		go func(src string, dst string) {
			defer wg.Done()
			result, err := h.syncFile(ctx, src, dst)
			if err != nil {
				errChan <- fmt.Errorf("failed to upload file %s in %s: %w", src, h.Type(), err)
				return
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(candidate, bucketDest)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return results, nil
}
