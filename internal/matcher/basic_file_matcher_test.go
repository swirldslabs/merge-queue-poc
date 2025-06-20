package matcher

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.hedera.com/solo-cheetah/internal/config"
)

func TestBasicFileMatcher_MatchFiles(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	markerName := "test_marker.rcd_sig"
	markerPath := filepath.Join(tempDir, markerName)
	require.NoError(t, os.WriteFile(markerPath, []byte("marker"), 0644))

	files := []string{
		"test_marker.rcd.gz",
		"sidecar/test_marker_01.gz",
		"sidecar/test_marker_02.gz",
		"sidecar/test_marker_099.gz", // this shouldn't be pickup as it is out of sequence
		"sidecar/test_marker_1.gz",
		"sidecar/test_marker_2.gz",
		"sidecar/test_marker_99.gz", // this shouldn't be pickup as it is out of sequence
	}
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "sidecar"), 0755))
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, f), []byte("data"), 0644))
	}

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{
			name:     "only data file and marker file",
			patterns: []string{".rcd.gz", ".rcd_sig"},
			want: []string{
				fmt.Sprintf("%s/test_marker.rcd.gz", tempDir),
				fmt.Sprintf("%s/test_marker.rcd_sig", tempDir),
			},
		},
		{
			name:     "no match",
			patterns: []string{".notfound"},
			want:     []string{},
		},
	}

	matcher := NewBasicFileMatcher()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.FileMatcherConfig{Patterns: tt.patterns}
			got, err := matcher.MatchFiles(markerPath, cfg)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}
