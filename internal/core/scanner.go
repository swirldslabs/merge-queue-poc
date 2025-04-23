package core

import (
	"context"
	"golang.hedera.com/solo-cheetah/internal/logx"
	"os"
	"path/filepath"
)

type scanner struct {
	id      string
	path    string
	pattern string
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

// Scan traverses the specified directory path to find files matching the configured pattern.
// It streams the results of the scan through a channel and sends any errors encountered to the provided error channel.
//
// Parameters:
//   - ctx: The context used to manage cancellation and timeouts for the scanning process.
//   - path: The root directory path to start the scan.
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
func (s *scanner) Scan(ctx context.Context, path string, ech chan<- error) <-chan ScannerResult {
	items := make(chan ScannerResult)
	go func() {
		defer close(items)
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					logx.As().Warn().
						Str("path", path).
						Str("scanner", s.Info()).
						Msg("File seems to have been deleted during scan, ignoring error...")
					return nil
				}

				logx.As().Err(err).
					Str("path", path).
					Str("scanner", s.Info()).
					Msg("Error in scanner")

				return err
			}

			if !info.Mode().IsRegular() || filepath.Ext(path) != s.pattern {
				return nil // ignore non-regular files and non-matching extensions
			}

			logx.As().Debug().
				Str("path", path).
				Str("scanner", s.Info()).
				Str("ext", filepath.Ext(path)).
				Str("pattern", s.pattern).
				Msg("Candidate file found")

			select {
			case items <- ScannerResult{Path: path, Info: info}:
				logx.As().Info().
					Str("path", path).
					Str("scanner", s.Info()).
					Msg("Candidate file added to queue")
			case <-ctx.Done():
				return nil
			}

			return nil
		})

		if err != nil {
			logx.As().Err(err).
				Str("path", path).
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

// NewScanner creates a new instance of the scanner with the specified configuration.
//
// Parameters:
//   - id: A unique identifier for the scanner instance.
//   - path: The root directory path where the scanner will start searching for files.
//   - pattern: The file extension pattern to match during the scan (e.g., ".txt").
//
// Returns:
//   - A Scanner instance configured with the provided parameters.
//   - An error if the scanner could not be created.
//
// Notes:
//   - The scanner uses the provided `path` and `pattern` to filter files during the scan process.
//   - Ensure that the `path` exists and is accessible before using the scanner.
func NewScanner(id string, path string, pattern string) (Scanner, error) {
	return &scanner{id: id, path: path, pattern: pattern}, nil
}
