package matcher

import (
	"bytes"
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"path/filepath"
	"text/template"
)

var FileMatcherGlob = "glob"

type globFileMatcher struct {
}

func (gm *globFileMatcher) Type() string {
	return FileMatcherGlob
}

func (gm *globFileMatcher) MatchFiles(marker string, cfg config.FileMatcherConfig) ([]string, error) {
	if len(cfg.Patterns) == 0 {
		return []string{}, nil
	}

	markerDir, markerName, _ := fsx.SplitFilePath(marker)

	// determine the matchers
	var candidatePatterns []string
	for _, tmplStr := range cfg.Patterns {
		var candidatePattern string
		if core.IsFileExtension(tmplStr) { // it is a file extension like .rcd.gz or .log
			candidatePattern = fsx.CombineFilePath(markerDir, markerName, tmplStr)
		} else { // try it is as a template
			tmpl, err := template.New("pattern").Parse(tmplStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse template for file extension '%s': %w", tmplStr, err)
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, map[string]string{
				TemplateVarMarkerName: markerName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to execute template for file extension '%s': %w", tmplStr, err)
			}

			candidatePattern = filepath.Join(markerDir, buf.String())
		}

		candidatePatterns = append(candidatePatterns, candidatePattern)

	}

	return fsx.MatchFilePatterns(markerDir, candidatePatterns, 1024)
}

func NewGlobFileMatcher() FileMatcher {
	return &globFileMatcher{}
}
