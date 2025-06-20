package fsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchFilePatterns(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	files := []string{
		"foo.rcd_sig",
		"foo.rcd",
		"bar.txt",
		"baz.rcd_sig",
		"subdir/qux.rcd_sig",
	}
	// Create files and subdir
	for _, f := range files {
		fullPath := filepath.Join(tempDir, f)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		assert.NoError(t, os.WriteFile(fullPath, []byte("test"), 0644))
	}

	// Match *.rcd_sig anywhere
	matches, err := MatchFilePatterns(tempDir, []string{"**/*.rcd_sig"}, 10)
	assert.NoError(t, err)

	expected := map[string]struct{}{
		filepath.Join(tempDir, "foo.rcd_sig"):        {},
		filepath.Join(tempDir, "baz.rcd_sig"):        {},
		filepath.Join(tempDir, "subdir/qux.rcd_sig"): {},
	}
	assert.Equal(t, len(expected), len(matches))
	for _, m := range matches {
		_, ok := expected[m]
		assert.True(t, ok, "unexpected match: %s", m)
	}
}
