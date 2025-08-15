package sniff

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Mock dependencies
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info() *MockLogger {
	return m
}

func (m *MockLogger) Trace() *MockLogger {
	return m
}

func (m *MockLogger) Error() *MockLogger {
	return m
}

func (m *MockLogger) Msg(msg string) {
	m.Called(msg)
}

func TestSniffer_StartAndStop(t *testing.T) {
	opts := &ProfilingConfig{
		Enabled:           true,
		EnablePprofServer: false,
		Interval:          "1s",
		Directory:         t.TempDir(),
		MaxSize:           1,
	}

	s := &Sniffer{
		opts: opts,
	}

	// Start the sniffer
	ctx := context.Background()
	err := s.Start(ctx)
	assert.NoError(t, err)

	// Stop the sniffer
	s.Stop()
}

func TestSniffer_CaptureStats(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	opts := &ProfilingConfig{
		Enabled:   true,
		Directory: tempDir,
		Interval:  "1s",
		MaxSize:   1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Sniffer{
		opts:   opts,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start capturing stats
	go func() {
		err := s.startCapturingStats()
		assert.NoError(t, err)
	}()

	// Wait for stats file to be created
	time.Sleep(2 * time.Second)

	statsFile := filepath.Join(tempDir, "stats.json")
	_, err := os.Stat(statsFile)
	assert.NoError(t, err)

	// Stop capturing stats
	s.Stop()
}

func TestSniffer_RotateFileIfNeeded(t *testing.T) {
	tempDir := t.TempDir()
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	statsFile := filepath.Join(tempDir, "stats.json")
	opts := &ProfilingConfig{
		Directory: tempDir,
		MaxSize:   1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Sniffer{
		opts:   opts,
		ctx:    ctx,
		cancel: cancel,
	}

	// Create a new file
	f, encoder, err := s.rotateFileIfNeeded(nil, nil, statsFile, opts.MaxSize)
	assert.NoError(t, err)

	// write some data
	err = os.WriteFile(statsFile, []byte("test data"), 0644)
	assert.NoError(t, err)

	// move s.nextRotationHour to the past
	nextRotationHour := time.Now().Add(-time.Hour)
	s.nextRotationHour = &nextRotationHour

	// Rotate the file
	f, encoder, err = s.rotateFileIfNeeded(f, encoder, statsFile, 0)
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.NotNil(t, encoder)

	// Verify the old file was rotated
	files, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Greater(t, len(files), 1)
}

func TestSniffer_WriteStatsToFile(t *testing.T) {
	tempDir := t.TempDir()
	statsFile := filepath.Join(tempDir, "stats.json")
	f, err := os.Create(statsFile)
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
		_ = os.RemoveAll(tempDir)
	}()

	encoder := json.NewEncoder(f)

	ctx, cancel := context.WithCancel(context.Background())
	s := &Sniffer{
		ctx:    ctx,
		cancel: cancel,
	}

	memStats := &MemStats{AllocMiB: 10, SysMiB: 30, NumGC: 5}
	cpuStats := &CPUStats{NumGoroutines: 10, NumCPU: 4, NumCgoCalls: 100}

	// Write stats to file
	err = s.writeStatsToFile(encoder, memStats, cpuStats)
	assert.NoError(t, err)

	// Verify the file content
	content, err := os.ReadFile(statsFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), `"alloc_mib"`)
	assert.Contains(t, string(content), `"num_gc"`)
}
