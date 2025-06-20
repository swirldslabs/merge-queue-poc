package processor

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/matcher"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"sort"
	"sync"
	"time"
)

// DefaultDelayBeforeUpload is the default delay before uploading files to allow flushing data files.
const DefaultDelayBeforeUpload = time.Millisecond * 150
const DefaultMarkerFileCheckInterval = time.Millisecond * 100
const DefaultMarkerFileCheckMaxAttempts = 3
const DefaultMarkerFileCheckMinSize = 0 // default minimum size for marker files to be considered ready

type processor struct {
	id                 string
	storages           []core.Storage
	fileMatcherConfigs []config.FileMatcherConfig
	flushDelay         time.Duration     // delay before uploading files to allow flushing data files
	markerCheckConfig  markerCheckConfig // configuration for marker file checks

}

// markerCheckConfig holds the configuration for checking marker files before processing them.
// We define a separate struct so that we can translate config.CheckerConfig to markerCheckConfig particularly parsing
// the checkInterval as time.Duration.
type markerCheckConfig struct {
	checkInterval time.Duration
	maxAttempts   int
	minSize       int64
}

func (p *processor) Info() string {
	return p.id
}

// Process orchestrates the processing of files by managing their upload and subsequent removal.
// It receives files from the scanner through the input channel and processes them using the configured storage handlers.
// The function ensures that errors encountered during the process are sent to the provided error channel.
//
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the processing pipeline.
//   - markers: A channel that streams the files to be processed and uploaded.
//   - ech: A channel to which errors encountered during processing are sent.
//
// Behavior:
//   - Files are first uploaded using the `upload` method, which handles parallel uploads to storage handlers.
//   - After successful uploads, files are removed locally using the `remove` method.
//   - Any errors encountered during upload or removal are sent to the error channel.
//
// Notes:
//   - The function terminates processing if the context is canceled.
func (p *processor) Process(ctx context.Context, markers <-chan core.ScannerResult, ech chan<- error) {
	logx.As().Trace().Msg("Processor starting")

	// setup process pipeline
	stored := p.upload(ctx, markers)
	sch := p.remove(ctx, stored)

	// copy errors to channel
	for err := range sch {
		if err != nil {
			select {
			case ech <- err:
			case <-ctx.Done():
				return
			}
		}
	}
	logx.As().Trace().Msg("Processing stopped")
}

// waitForMarkerFileToBeReady checks if the marker file is ready for processing by ensuring it has reached the minimum size.
// It applies a delay before checking the file size to allow for flushing data files.
// If the marker file is not ready, it will repeatedly check its size until it reaches the minimum size or the maximum number of attempts is reached.
// If the marker file is already larger than the minimum size, it returns immediately.
// If the marker file does not exist, it returns an error.
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the waiting process.
//   - marker: The ScannerResult containing the path and file info of the marker file.
//
// Returns:
//   - os.FileInfo of the marker file if it is ready, or an error if the file does not exist or other issues occur.
func (p *processor) waitForMarkerFileToBeReady(ctx context.Context, marker core.ScannerResult) (os.FileInfo, error) {
	// if the marker file is already larger than the minimum size, we can skip waiting
	if marker.Info != nil && marker.Info.Size() >= p.markerCheckConfig.minSize {
		logx.As().Debug().
			Str("marker", marker.Path).
			Int64("size", marker.Info.Size()).
			Int64("min_size", p.markerCheckConfig.minSize).
			Msg("Marker file is ready for processing already, no need to wait")
		return marker.Info, nil
	}

	// apply a delay to allow flushing data files before checking marker file
	if p.flushDelay > 0 {
		core.ApplyDelay(ctx, p.flushDelay)
	}

	// we will check the marker file size until it reaches the minimum size; or we reach the maximum number of attempts
	attempts := 0
	markerPath := marker.Path
	markerInfo := marker.Info
	var err error
	for {
		select {
		case <-ctx.Done():
			logx.As().Warn().Msg("Processor context cancelled while waiting for marker file to be ready")
			return nil, context.Canceled
		default:
			if attempts >= p.markerCheckConfig.maxAttempts {
				logx.As().Warn().
					Str("marker", markerPath).
					Int("attempts", attempts).
					Int("max_attempts", p.markerCheckConfig.maxAttempts).
					Msg("MarkerPath file is not ready after maximum attempts, continuing with upload anyway")
				return markerInfo, nil // even if the marker file size does not reach the minimum size, we continue with upload
			}

			markerInfo, err = os.Stat(markerPath)
			if err == nil {
				if markerInfo.Size() >= p.markerCheckConfig.minSize {
					logx.As().Trace().
						Str("marker", markerPath).
						Int64("size", markerInfo.Size()).
						Int64("min_size", p.markerCheckConfig.minSize).
						Int("attempts", attempts).
						Int("max_attempts", p.markerCheckConfig.maxAttempts).
						Msg("MarkerPath file is ready for processing")
					return markerInfo, nil
				}

				logx.As().Warn().
					Str("marker", markerPath).
					Int64("size", markerInfo.Size()).
					Int64("min_size", p.markerCheckConfig.minSize).
					Int("attempts", attempts).
					Int("max_attempts", p.markerCheckConfig.maxAttempts).
					Msg("MarkerPath file is not ready, waiting for it to reach minimum size")
			} else {
				// this can happen if the file was removed
				return nil, fmt.Errorf("marker file doesn't exist %s: %w", markerPath, err)
			}

			core.ApplyDelay(ctx, p.markerCheckConfig.checkInterval)
			attempts++
		}
	}
}

// upload handles the parallel uploading of files to the configured storage handlers.
// It receives files from the scanner through the input channel and processes them using goroutines
// to upload to multiple storage handlers concurrently. The method ensures that the results of the
// upload operations are sent through the returned channel.
//
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the upload process.
//   - items: A channel that streams the marker files to be processed.
//
// Returns:
//   - A channel of ProcessorResult, which contains the results of the upload operations for each file.
//
// Behavior:
//   - For each file, the method checks if the file exists locally before processing.
//   - Uploads are performed in parallel for all configured storage handlers using goroutines.
//   - UploadResults from all storage handlers are accumulated into a ProcessorResult object.
//   - If any storage operation fails, the first error is recorded in the ProcessorResult, and it returns from the method.
//   - The method terminates processing if the context is canceled.
//
// Notes:
//   - The returned channel is closed after all files have been processed or the context is cancelled.
//   - fail fast if any storage operation failed
func (p *processor) upload(ctx context.Context, markers <-chan core.ScannerResult) <-chan core.ProcessorResult {
	logx.As().Trace().Msg("Processor upload starting")

	processed := make(chan core.ProcessorResult)
	go func() {
		defer close(processed)
		for marker := range markers {
			select {
			case <-ctx.Done():
				logx.As().Warn().Msg("Processor context cancelled, stopping uploading files")
			default:
				if _, exists := fsx.PathExists(marker.Path); !exists {
					continue
				}

				var err error
				marker.Info, err = p.waitForMarkerFileToBeReady(ctx, marker)
				if err != nil {
					logx.As().Warn().
						Err(err).
						Str("marker", marker.Path).
						Str("trace_id", marker.TraceId).
						Msg("Failed to wait for marker file to be ready, skipping upload")
					continue // skip this file if it is not ready
				}

				stored := make(chan core.StorageResult) // shared channel to receive storage results, closed after all storages are done
				pr := core.ProcessorResult{
					Error:   nil,
					Path:    marker.Path,
					TraceId: marker.TraceId,
					Result:  make(map[string]*core.StorageResult),
				}

				candidates, err := p.prepareUploadCandidates(marker.Path)
				if err != nil {
					logx.As().Warn().
						Err(err).
						Str("marker", marker.Path).
						Str("trace_id", marker.TraceId).
						Msg("Failed to prepare upload candidates, skipping upload")
					continue // skip this file if we cannot prepare candidates
				}

				logx.As().Info().
					Str("marker", marker.Path).
					Str("trace_id", marker.TraceId).
					Int64("marker_file_size", marker.Info.Size()).
					Str("candidates", fmt.Sprintf("%v", candidates)).
					Msg("Processor processing marker file")

				// parallel upload
				var wg sync.WaitGroup
				for _, storage := range p.storages {
					wg.Add(1)
					go func(s core.Storage) {
						defer wg.Done()
						s.Put(ctx, marker, candidates, stored)
					}(storage)
				}

				// Wait for all storages to finish storing
				go func() {
					wg.Wait()
					close(stored) // Close the channel after all storages are done
				}()

				// accumulate response from the storage handlers
				for resp := range stored {
					if resp.Error != nil {
						if pr.Error == nil {
							pr.Error = fmt.Errorf("%s: %s", resp.Error, resp.MarkerPath) // set the first error
						}
					}

					pr.Result[resp.Type] = &resp
				}

				// send to the channel
				select {
				case processed <- pr:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return processed
}

// remove handles the removal of local files after they have been successfully uploaded to remote storage.
// It processes the results of the upload operation and ensures that files with no errors are deleted locally.
// Any errors encountered during the removal process are sent to the provided error channel.
//
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the removal process.
//   - stored: A channel that streams the results of the upload operations.
//
// Returns:
//   - A channel of errors, which contains any errors encountered during the file removal process.
//
// Behavior:
//   - For each file, if the upload was successful, the local file is removed.
//   - If an error occurs during the removal, it is sent to the error channel.
//   - The function terminates processing if the context is canceled.
//
// Notes:
//   - Files with upload errors are skipped and not removed.
//   - The returned error channel is closed after all files have been processed or if the context is canceled.
func (p *processor) remove(ctx context.Context, stored <-chan core.ProcessorResult) <-chan error {
	sch := make(chan error, 1)
	go func() {
		defer close(sch)
		for resp := range stored {
			select {
			case <-ctx.Done():
				logx.As().Warn().
					Str("marker", resp.Path).
					Str("trace_id", resp.TraceId).
					Str("processor", p.Info()).
					Msg("Processor context cancelled, stopping file removal")
				return
			default:
				if resp.Error != nil {
					if resp.Error != nil {
						logx.As().Warn().
							Err(resp.Error).
							Str("processor", p.Info()).
							Str("marker", resp.Path).
							Str("trace_id", resp.TraceId).
							Msg("One or more storage sync operation has failed. Skipping file removal")
					}

					select {
					case sch <- resp.Error:
					case <-ctx.Done():
						return
					}
					continue // skip file removal if there was an error
				}

				removalCandidates := p.prepareRemovalCandidates(resp)

				logx.As().Info().
					Str("marker", resp.Path).
					Str("trace_id", resp.TraceId).
					Str("processor", p.Info()).
					Str("local_files", fmt.Sprintf("%v", removalCandidates)).
					Msg("Processor processed marker file, removing local copies")

				for _, pathToRemove := range removalCandidates {
					if _, exists := fsx.PathExists(pathToRemove); exists {
						err := os.Remove(pathToRemove)
						if err != nil {
							logx.As().
								Err(err).
								Str("trace_id", resp.TraceId).
								Str("path", pathToRemove).
								Msg("Failed to remove file")
							select {
							case sch <- err:
							case <-ctx.Done():
								return
							}
						}
						logx.As().Info().
							Str("path", pathToRemove).
							Str("trace_id", resp.TraceId).
							Str("processor", p.Info()).
							Msg("Removed local file after successful upload")
					}
				}
			}
		}
	}()
	return sch
}

func (p *processor) prepareUploadCandidates(marker string) ([]string, error) {
	var candidates []string
	for _, mc := range p.fileMatcherConfigs {
		m, err := matcher.GetFileMatcher(mc.MatcherType)
		if err != nil {
			return nil, fmt.Errorf("unknown file matcher type: %s", mc.MatcherType)
		}

		matches, err := m.MatchFiles(marker, mc)
		if err != nil {
			return nil, fmt.Errorf("failed to match files for marker %s: %w", marker, err)
		}

		logx.As().Debug().
			Str("marker", marker).
			Str("matcher", m.Type()).
			Str("matches", fmt.Sprintf("%v", matches)).
			Str("patterns", fmt.Sprintf("%v", mc.Patterns)).
			Msg("Results of matcher")

		candidates = append(candidates, matches...)
	}

	return candidates, nil
}

func (p *processor) prepareRemovalCandidates(resp core.ProcessorResult) []string {
	uniqueCandidates := make(map[string]struct{})
	uniqueCandidates[resp.Path] = struct{}{}

	for _, storageResult := range resp.Result {
		if storageResult.Error != nil {
			continue // skip removal if there was an error
		}
		for _, info := range storageResult.UploadResults {
			if info != nil {
				uniqueCandidates[info.Src] = struct{}{}
			}
		}
	}

	removalCandidates := make([]string, 0, len(uniqueCandidates))
	for path := range uniqueCandidates {
		removalCandidates = append(removalCandidates, path)
	}

	sort.Strings(removalCandidates) // ensure deterministic order

	return removalCandidates
}

func NewProcessor(id string, storages []core.Storage, pc *config.ProcessorConfig) (core.Processor, error) {
	flushDelay := DefaultDelayBeforeUpload
	var err error
	if pc.FlushDelay != "" {
		flushDelay, err = time.ParseDuration(pc.FlushDelay)
		if err != nil {
			return nil, fmt.Errorf("failed to parse flushDelay: %w", err)
		}
	}

	mc := markerCheckConfig{
		checkInterval: DefaultMarkerFileCheckInterval,
		maxAttempts:   DefaultMarkerFileCheckMaxAttempts,
		minSize:       DefaultMarkerFileCheckMinSize,
	}

	if pc.MarkerCheckConfig != nil {
		if pc.MarkerCheckConfig.CheckInterval != "" {
			mc.checkInterval, err = time.ParseDuration(pc.MarkerCheckConfig.CheckInterval)
			if err != nil {
				return nil, fmt.Errorf("failed to parse marker file check interval: %w", err)
			}
		}
		mc.minSize = pc.MarkerCheckConfig.MinSize
		mc.maxAttempts = pc.MarkerCheckConfig.MaxAttempts
	}

	return newProcessor(id, storages, pc.FileMatcherConfigs, flushDelay, mc)
}

func newProcessor(id string, storages []core.Storage, fileMatchersConfigs []config.FileMatcherConfig, flushDelay time.Duration, mc markerCheckConfig) (*processor, error) {
	return &processor{
		id:                 id,
		storages:           storages,
		fileMatcherConfigs: fileMatchersConfigs,
		flushDelay:         flushDelay,
		markerCheckConfig:  mc,
	}, nil
}
