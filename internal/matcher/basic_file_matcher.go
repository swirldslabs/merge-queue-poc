package matcher

import (
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
)

var FileMatcherBasic = "basic"

// basicFileMatcher implements the FileMatcher interface for matching assuming Patterns only include file extensions.
// for example if Patterns is [".txt", ".json"], it will look for files like marker.txt and marker.json in the same
// directory as the marker file.
type basicFileMatcher struct {
	*defaultFileMatcher
}

// MatchFiles returns a list of file paths that match the given patterns for the provided marker file.
// It replaces the marker file's name in each pattern, checks if the resulting file exists in the marker's directory,
// and collects all existing matches. If no patterns are provided, it returns an empty slice.
func (bm *basicFileMatcher) MatchFiles(marker string, cfg config.FileMatcherConfig) ([]string, error) {
	if len(cfg.Patterns) == 0 {
		return []string{}, nil
	}

	markerDir, markerName, _ := fsx.SplitFilePath(marker)

	var matches []string

	for _, ext := range cfg.Patterns {
		if !core.IsFileExtension(ext) {
			return nil, fmt.Errorf("%s is not a valid file extension", ext)
		}

		candidateFile := fsx.CombineFilePath(markerDir, markerName, ext)
		logx.As().Debug().
			Str("matcher", bm.Type()).
			Str("file_pattern", ext).
			Str("candidate_file", candidateFile).
			Msg("Checking file")
		if _, err := os.Stat(candidateFile); err == nil {
			logx.As().Debug().
				Str("matcher", bm.Type()).
				Str("file_pattern", ext).
				Str("candidate_file", candidateFile).
				Msg("Found candidate file")
			matches = append(matches, candidateFile)
		}
	}

	return matches, nil
}

func NewBasicFileMatcher() FileMatcher {
	return &basicFileMatcher{
		defaultFileMatcher: &defaultFileMatcher{
			matcherType: FileMatcherBasic,
		},
	}
}
