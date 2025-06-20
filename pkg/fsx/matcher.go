package fsx

import (
	"fmt"
	"github.com/gobwas/glob"
	"os"
)

// MatchFilePatterns recursively walks the specified directory and returns a list of unique file paths
// that match any of the provided glob patterns. Only regular files are considered. The function compiles
// all patterns to glob matchers and applies them to each file found during traversal. If an error occurs
// during directory traversal or pattern compilation, it is returned.
//
// Parameters:
//
//	dir      - the root directory to search
//	patterns - a slice of glob pattern strings to match file paths
//	batchSize - the batch size for the directory walker
//
// Returns:
//
//	A slice of matching file paths and an error, if any occurred.
func MatchFilePatterns(dir string, patterns []string, batchSize int) ([]string, error) {
	foundMap := map[string]struct{}{}
	walker := NewWalker(batchSize)

	var matchers []glob.Glob
	for _, pattern := range patterns {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile glob pattern '%s': %w", pattern, err)
		}
		matchers = append(matchers, g)
	}

	err := walker.Start(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		for _, matcher := range matchers {
			if matcher.Match(path) {
				foundMap[path] = struct{}{}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory '%s': %w", dir, err)
	}

	matches := make([]string, 0, len(foundMap))
	for match := range foundMap {
		matches = append(matches, match)
	}

	return matches, err
}
