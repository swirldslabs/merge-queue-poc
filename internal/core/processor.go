package core

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"sync"
)

type processor struct {
	id             string
	storages       []Storage
	fileExtensions []string
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
//   - items: A channel that streams the files to be processed and uploaded.
//   - ech: A channel to which errors encountered during processing are sent.
//
// Behavior:
//   - Files are first uploaded using the `upload` method, which handles parallel uploads to storage handlers.
//   - After successful uploads, files are removed locally using the `remove` method.
//   - Any errors encountered during upload or removal are sent to the error channel.
//
// Notes:
//   - The function terminates processing if the context is canceled.
func (p *processor) Process(ctx context.Context, items <-chan ScannerResult, ech chan<- error) {
	logx.As().Debug().Msg("Processor starting")

	// setup process pipeline
	stored := p.upload(ctx, items)
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
}

// upload handles the parallel uploading of files to the configured storage handlers.
// It receives files from the scanner through the input channel and processes them using goroutines
// to upload to multiple storage handlers concurrently. The method ensures that the results of the
// upload operations are sent through the returned channel.
//
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the upload process.
//   - items: A channel that streams the files to be uploaded.
//
// Returns:
//   - A channel of ProcessorResult, which contains the results of the upload operations for each file.
//
// Behavior:
//   - For each file, the method checks if the file exists locally before processing.
//   - Uploads are performed in parallel for all configured storage handlers using goroutines.
//   - Results from all storage handlers are accumulated into a ProcessorResult object.
//   - If any storage operation fails, the first error is recorded in the ProcessorResult, and it returns from the method.
//   - The method terminates processing if the context is canceled.
//
// Notes:
//   - The returned channel is closed after all files have been processed or the context is cancelled.
//   - fail fast if any storage operation failed
func (p *processor) upload(ctx context.Context, items <-chan ScannerResult) <-chan ProcessorResult {
	logx.As().Debug().Msg("Processor starting")

	processed := make(chan ProcessorResult)
	go func() {
		defer close(processed)
		for item := range items {
			select {
			case <-ctx.Done():
				logx.As().Warn().Msg("Processor context cancelled, stopping uploading files")
			default:
				if _, exists := fsx.PathExists(item.Path); !exists {
					continue
				}

				stored := make(chan StorageResult) // shared channel to receive storage results, closed after all storages are done
				pr := ProcessorResult{
					Error:  nil,
					Path:   item.Path,
					Result: make(map[string]*StorageResult),
				}

				logx.As().Debug().Str("marker", item.Path).Msg("Processor processing marker file")

				// parallel upload
				var wg sync.WaitGroup
				for _, storage := range p.storages {
					wg.Add(1)
					go func(s Storage) {
						defer wg.Done()
						s.Put(ctx, item, stored)
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
							pr.Error = fmt.Errorf("%s: %s", resp.Error, resp.Src) // set the first error
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

				// fail fast if any storage operation failed
				if pr.Error != nil {
					logx.As().Warn().
						Err(pr.Error).
						Str("processor", p.Info()).
						Str("marker", item.Path).
						Msg("One or more storage sync operation has failed; stopping file upload...")
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
func (p *processor) remove(ctx context.Context, stored <-chan ProcessorResult) <-chan error {
	sch := make(chan error, 1)
	go func() {
		defer close(sch)
		for resp := range stored {
			select {
			case <-ctx.Done():
				logx.As().Warn().
					Str("marker", resp.Path).
					Str("processor", p.Info()).
					Msg("Processor context cancelled, stopping file removal")
				return
			default:
				if resp.Error != nil {
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
					Str("processor", p.Info()).
					Str("local_files", fmt.Sprintf("%v", removalCandidates)).
					Msg("Processor processed marker file, removing local copies")

				for _, pathToRemove := range removalCandidates {
					if _, exists := fsx.PathExists(pathToRemove); exists {
						err := os.Remove(pathToRemove)
						if err != nil {
							logx.As().
								Err(err).
								Str("path", pathToRemove).
								Msg("Failed to remove file")
							select {
							case sch <- err:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}()
	return sch
}

func (p *processor) prepareRemovalCandidates(resp ProcessorResult) []string {
	uniqueCandidates := make(map[string]bool)
	uniqueCandidates[resp.Path] = true

	dir, fileName, _ := fsx.SplitFilePath(resp.Path)
	for _, ext := range p.fileExtensions {
		path := fsx.CombineFilePath(dir, fileName, ext)
		uniqueCandidates[path] = true
	}

	var removalCandidates []string
	for path := range uniqueCandidates {
		removalCandidates = append(removalCandidates, path)
	}

	return removalCandidates
}

func NewProcessor(id string, storages []Storage, fileExtensions []string) (Processor, error) {
	// if pattern contains '*' or '?' in fileExtensions, it is not a supported pattern. We only allow extension like .rcd_sig without * or ?
	if len(fileExtensions) > 0 {
		for _, ext := range fileExtensions {
			if !config.IsValidExtension(ext) {
				return nil, fmt.Errorf("invalid file extension '%s'. use file extension without * or regex characters; i.e. '.rcd.gz'", ext)
			}
		}
	}

	return &processor{
		id:             id,
		storages:       storages,
		fileExtensions: fileExtensions,
	}, nil
}
