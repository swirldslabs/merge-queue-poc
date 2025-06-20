package matcher

import (
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var FileMatcherSequential = "sequential"

type sequentialFileMatcher struct{}

func (sm *sequentialFileMatcher) Type() string {
	return FileMatcherSequential
}

// MatchFiles returns a list of sidecar file paths that match the given patterns for a marker file.
// It supports patterns with digit placeholders:
//   - "##.gz": matches files with two-digit sequence numbers (e.g., marker_01.gz, marker_02.gz, ...)
//   - "#.gz": matches files with one-digit sequence numbers (e.g., marker_1.gz, marker_2.gz, ...)
//
// For each pattern, files are collected in sequence starting from 1, stopping at the first missing file.
// The returned paths are relative to the marker's directory.
// If no patterns are provided, it returns an empty slice.
func (sm *sequentialFileMatcher) MatchFiles(marker string, cfg config.FileMatcherConfig) ([]string, error) {
	if len(cfg.Patterns) == 0 {
		return []string{}, nil
	}

	markerDir, markerName, _ := fsx.SplitFilePath(marker)

	var matches []string
	re := regexp.MustCompile(`#+`)

	for _, pattern := range cfg.Patterns {
		pattern = strings.ReplaceAll(pattern, "{{.markerName}}", markerName)

		// Replace sequences of # with corresponding %0Nd
		isSequenced := false
		filePattern := re.ReplaceAllStringFunc(pattern, func(s string) string {
			isSequenced = true
			return fmt.Sprintf("%%0%dd", len(s))
		})

		if !isSequenced {
			candidateFile := filepath.Join(markerDir, filePattern)
			if _, err := os.Stat(candidateFile); err == nil {
				matches = append(matches, candidateFile)
			}
			continue
		}

		// search for linearly named files like file_01, file_02, etc.
		var i int
		for {
			fileName := fmt.Sprintf(filePattern, i+1)
			candidateFile := filepath.Join(markerDir, fileName)
			logx.As().Debug().
				Str("matcher", sm.Type()).
				Str("file_pattern", filePattern).
				Str("candidate_file", candidateFile).
				Msg("Checking if candidate file exists")
			if _, err := os.Stat(candidateFile); err == nil {
				logx.As().Debug().
					Str("matcher", sm.Type()).
					Str("file_pattern", filePattern).
					Str("candidate_file", candidateFile).
					Msg("Found candidate file")
				matches = append(matches, candidateFile)
				i++
			} else {
				break // Stop searching if a file does not exist, as files are sequentially named.
			}
		}
	}

	return matches, nil
}

func NewSidecarFileMatcher() FileMatcher {
	return &sequentialFileMatcher{}
}
