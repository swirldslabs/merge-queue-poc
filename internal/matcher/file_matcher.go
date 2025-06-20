package matcher

import (
	"fmt"
	"golang.hedera.com/solo-cheetah/internal/config"
)

var fileMatchers map[string]FileMatcher

func RegisterFileMatcher(fm FileMatcher) {
	if fileMatchers == nil {
		fileMatchers = map[string]FileMatcher{}
	}

	fileMatchers[fm.Type()] = fm
}

func GetFileMatcher(matcherType string) (FileMatcher, error) {
	if fm, ok := fileMatchers[matcherType]; ok {
		return fm, nil
	}
	return nil, fmt.Errorf("file matcher %s not found", matcherType)
}

type FileMatcher interface {
	Type() string
	MatchFiles(marker string, cfg config.FileMatcherConfig) ([]string, error)
}

type defaultFileMatcher struct {
	matcherType string
}

func (dfm *defaultFileMatcher) Type() string {
	return dfm.matcherType
}
