package core

import (
	"context"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
)

type scanner struct {
	id        string
	directory string
	pattern   string
	walker    *fsx.Walker
}

// Info returns a unique identifier for the scanner or processor instance.
//
// Returns:
//   - A string representing the unique identifier of the instance.
//
// Notes:
//   - This method is typically used for logging and debugging purposes to distinguish between different instances.
func (s *scanner) Info() string {
	return s.id
}

// Scan traverses the specified directory to find files matching the configured pattern.
// It streams the results of the scan through a channel and sends any errors encountered to the provided error channel.
//
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the scanning process.
//   - ech: A channel to which errors encountered during the scan are sent.
//
// Returns:
//   - A channel of ScannerResult, which streams the details of the files that match the configured pattern.
//
// Behavior:
//   - The method uses `filepath.Walk` to recursively traverse the directory tree.
//   - Files that are not regular or do not match the specified pattern are ignored.
//   - If a file is deleted during the scan, the error is logged and ignored.
//   - If the context is canceled, the scan stops immediately.
//
// Notes:
//   - The returned channel is closed after all matching files have been processed or if the context is canceled.
//   - Errors encountered during the scan are sent to the error channel but do not stop the scanning process.
func (s *scanner) Scan(ctx context.Context, ech chan<- error) <-chan ScannerResult {
	items := make(chan ScannerResult)
	go func() {
		defer s.walker.End()
		defer close(items)
		err := s.walker.Start(s.directory, func(path string, info os.FileInfo, err error) error {
			logx.As().Debug().Str("path", path).Msg("scanning path")

			if err != nil {
				if os.IsNotExist(err) {
					logx.As().Warn().
						Str("path", path).
						Str("scanner", s.Info()).
						Msg("Path doesn't exists, skipping path...")
					return nil
				}

				logx.As().Err(err).
					Str("path", path).
					Str("scanner", s.Info()).
					Msg("Error in scanner")

				return err
			}

			ext := filepath.Ext(path)
			if !info.Mode().IsRegular() || ext != s.pattern {
				logx.As().Debug().
					Str("path", path).
					Str("ext", ext).
					Str("marker_pattern", s.pattern).
					Str("mode", info.Mode().String()).
					Bool("is_regular", info.Mode().IsRegular()).
					Msg("skipping path")
				return nil // ignore non-regular files and non-matching extensions
			}

			logx.As().Info().
				Str("path", path).
				Str("scanner", s.Info()).
				Str("ext", filepath.Ext(path)).
				Str("pattern", s.pattern).
				Msg("Scanner found marker file")

			select {
			case items <- ScannerResult{Path: path, Info: info}:
				logx.As().Debug().
					Str("marker", path).
					Str("scanner", s.Info()).
					Msg("Scanner added marker file to the queue")
			case <-ctx.Done():
				return nil
			}

			return nil
		})

		if err != nil {
			logx.As().Err(err).
				Str("directory", s.directory).
				Str("scanner", s.Info()).
				Msg("Error in scanner")
			select {
			case ech <- err:
			case <-ctx.Done():
			}
		}
	}()

	return items
}

// NewScanner creates and initializes a new scanner instance.
//
// Parameters:
//   - id: A unique identifier for the scanner instance.
//   - directory: The root directory directory to scan.
//   - pattern: The file extension pattern to match (e.g., ".txt").
//   - batchSize: The maximum number of directory entries to read at once.
//
// Returns:
//   - A Scanner instance configured with the provided parameters.
//   - An error if the scanner cannot be initialized.
//
// Notes:
//   - The scanner uses a Walker to traverse the directory tree.
//   - The batchSize parameter controls how many directory entries are read in a single operation.
func NewScanner(id string, rootDir string, pattern string, batchSize int) (Scanner, error) {
	return newScanner(id, rootDir, pattern, batchSize)
}

func newScanner(id string, rootDir string, pattern string, batchSize int) (*scanner, error) {
	// if pattern contains '*' or '?', it is not a supported pattern. We only allow extension like .rcd_sig
	if !config.IsValidExtension(pattern) {
		return nil, fmt.Errorf("invalid file extension '%s'. use file extension without * or regex characters; i.e. '.rcd.gz'", pattern)
	}

	return &scanner{
		id:        id,
		directory: rootDir,
		pattern:   pattern,
		walker:    fsx.NewWalker(batchSize),
	}, nil
}
