package core

import (
	"context"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"path/filepath"
	"strings"
	"time"
)

// IsFileExtension checks if the provided string is a valid file extension.
// for now, we only support extension begin with dot. We may support glob pattern or regex later
func IsFileExtension(ext string) bool {
	if len(ext) > 0 && ext[0] != '.' {
		return false
	}

	return true
}

// ComputeDestinationBucketPath computes the destination bucket path for a source file.
func ComputeDestinationBucketPath(rootDir string, srcFile string, bucketPathPrefix string) string {
	srcDir, fileName, ext := fsx.SplitFilePath(srcFile)
	relSubDirs := strings.TrimPrefix(filepath.Clean(srcDir), filepath.Clean(rootDir))
	destDir := filepath.Join(bucketPathPrefix, relSubDirs)
	dest := fsx.CombineFilePath(destDir, fileName, ext)
	return filepath.Clean(dest)
}

// ApplyDelay applies a delay to the execution of the current context.
func ApplyDelay(ctx context.Context, delay time.Duration) {
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop() // Ensure the timer is stopped to release resources

		select {
		case <-timer.C:
			// proceed after delay
		case <-ctx.Done():
			logx.As().Warn().Msg("context cancelled during delay")
		}
	}
}
