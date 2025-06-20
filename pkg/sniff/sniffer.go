package sniff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"
	"sync"
	"time"
)

var (
	sniffer     *Sniffer
	snifferOnce sync.Once
)

type Sniffer struct {
	opts             *ProfilingConfig
	nextRotationHour *time.Time
	lastSnapshot     *Stats
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.Mutex
}

func New(opts ProfilingConfig) *Sniffer {
	return &Sniffer{
		opts:             &opts,
		nextRotationHour: nil,
	}
}

// Start initializes and starts the global sniffer.
func Start(ctx context.Context, opts ProfilingConfig) error {
	var startErr error

	snifferOnce.Do(func() {
		sniffer = New(opts)
		startErr = sniffer.Start(ctx)
	})

	return startErr
}

// Stop stops the global sniffer if it is running.
func Stop() {
	if sniffer != nil {
		sniffer.Stop()
		sniffer = nil
	}
}

// Get returns the global sniffer instance.
func Get() *Sniffer {
	return sniffer
}

func (s *Sniffer) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("sniffer context is nil")
	}

	if !s.opts.Enabled {
		return nil
	}

	// create a new context from the parent context with cancel to manage the lifecycle of the sniffer components
	// if parent context is canceled, the sniffer will stop all its components
	s.ctx, s.cancel = context.WithCancel(ctx)

	now := time.Now()
	if s.nextRotationHour == nil {
		nextRotation := now.Truncate(time.Hour).Add(time.Hour)
		s.nextRotationHour = &nextRotation
	}

	if err := s.startSnapshotServer(); err != nil {
		return err
	}

	if s.opts.EnablePprofServer && s.opts.PprofPort > 0 {
		// Start pprof server if enabled
		go func() {
			pprofAddr := fmt.Sprintf("%s:%d", s.opts.ServerHost, s.opts.PprofPort)
			logx.As().Info().Msg(fmt.Sprintf("Starting pprof server on %s", pprofAddr))
			if err := http.ListenAndServe(pprofAddr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logx.As().Error().Err(err).Msg("pprof server failed")
			}
		}()

	}

	go func() {
		<-ctx.Done() // check if the parent context is canceled
		logx.As().Trace().Msg("Context canceled, stopping profiling data capture...")
		s.Stop()
	}()

	return s.startCapturingStats()
}

func (s *Sniffer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// startSnapshotServer starts an HTTP server to serve the last captured profiling data.
// It is closed once the sniffer context is canceled.
func (s *Sniffer) startSnapshotServer() error {
	if s.ctx == nil {
		return fmt.Errorf("sniffer context is nil")
	}
	serverURL := fmt.Sprintf("%s:%d", s.opts.ServerHost, s.opts.ServerPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/last-snapshot", func(w http.ResponseWriter, r *http.Request) {
		if s.lastSnapshot == nil {
			http.Error(w, "Stats not available", http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(s.lastSnapshot); err != nil {
			http.Error(w, "Failed to encode profiling data", http.StatusInternalServerError)
		}
	})

	server := &http.Server{
		Addr:    serverURL,
		Handler: mux,
	}

	go func() {
		logx.As().Info().Msg(fmt.Sprintf("Starting snapshot server on %s", serverURL))
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logx.As().Error().Err(err).Msg("snapshot server failed")
		}
	}()

	go func(sv *http.Server) {
		<-s.ctx.Done()
		logx.As().Info().Msg("Shutting down snapshot server...")
		if err := sv.Shutdown(context.Background()); err != nil {
			logx.As().Error().Err(err).Msg("Failed to shut down snapshot server")
		}
	}(server)

	return nil
}

// startCapturingStats starts capturing runtime profiling data and writing them to a file.
// It is closed once the sniffer context is canceled.
func (s *Sniffer) startCapturingStats() error {
	if s.ctx == nil {
		return fmt.Errorf("sniffer context is nil")
	}

	if err := os.MkdirAll(s.opts.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	interval, err := time.ParseDuration(s.opts.Interval)
	if err != nil {
		return fmt.Errorf("error parsing watch interval: %w", err)
	}

	statsFile := path.Join(s.opts.Directory, "stats.json")
	var f *os.File
	var encoder *json.Encoder
	defer fsx.CloseFile(f)

	maxFileSize := int64(s.opts.MaxSize) * 1024 * 1024
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				time.Sleep(interval)

				f, encoder, err = s.rotateFileIfNeeded(f, encoder, statsFile, maxFileSize)
				if err != nil {
					logx.As().Error().Err(err).Msg("Failed to handle stats file")
					continue
				}

				memStats, cpuStats := s.collectStats()
				if err := s.writeStatsToFile(encoder, memStats, cpuStats); err != nil {
					logx.As().Error().Err(err).Msg("Failed to write stats")
				}

				logx.As().Info().
					Uint64("Alloc(MiB)", memStats.AllocMiB).
					Uint64("TotalAlloc(MiB)", memStats.TotalAllocMiB).
					Uint64("Sys(MiB)", memStats.SysMiB).
					Uint32("NumGC", memStats.NumGC).
					Int("NumGoroutines", cpuStats.NumGoroutines).
					Int("NumCPU", cpuStats.NumCPU).
					Int64("NumCgoCalls", cpuStats.NumCgoCalls).
					Msg("Captured runtime profiling data")
			}
		}
	}()

	return nil
}

func (s *Sniffer) rotateFileIfNeeded(f *os.File, encoder *json.Encoder, filePath string, maxSize int64) (*os.File, *json.Encoder, error) {
	now := time.Now()

	if f == nil || encoder == nil {
		return s.createNewFile(filePath)
	}

	if now.After(*s.nextRotationHour) {
		if err := f.Close(); err != nil {
			return nil, nil, fmt.Errorf("failed to close current file: %w", err)
		}

		info, err := os.Stat(filePath)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("failed to stat file: %w", err)
		}

		if info.Size() >= maxSize {
			dir, fileName, ext := fsx.SplitFilePath(filePath)
			backupFileName := fmt.Sprintf("%s-%s", fileName, now.Format("2006-01-02T15-04-05.204"))
			backupPath := fsx.CombineFilePath(dir, backupFileName, ext)
			if err := os.Rename(filePath, backupPath); err != nil {
				return nil, nil, fmt.Errorf("failed to rotate file: %w", err)
			}
			logx.As().Info().Msg(fmt.Sprintf("Rotated stats file to %s", backupPath))
		}

		return s.createNewFile(filePath)
	}

	return f, encoder, nil
}

func (s *Sniffer) createNewFile(filePath string) (*os.File, *json.Encoder, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new stats file: %w", err)
	}

	nextRotation := time.Now().Truncate(time.Hour).Add(time.Hour)
	s.nextRotationHour = &nextRotation

	return f, json.NewEncoder(f), nil
}

func (s *Sniffer) collectStats() (*MemStats, *CPUStats) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memStats := &MemStats{
		AllocMiB:      m.Alloc / 1024 / 1024,
		TotalAllocMiB: m.TotalAlloc / 1024 / 1024,
		SysMiB:        m.Sys / 1024 / 1024,
		NumGC:         m.NumGC,
	}

	cpuStats := &CPUStats{
		NumGoroutines: runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		NumCgoCalls:   runtime.NumCgoCall(),
	}

	return memStats, cpuStats
}

func (s *Sniffer) writeStatsToFile(encoder *json.Encoder, memStats *MemStats, cpuStats *CPUStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSnapshot = &Stats{
		Pid:       logx.GetPid(),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		MemStats:  memStats,
		CPUStats:  cpuStats,
	}

	if err := encoder.Encode(s.lastSnapshot); err != nil {
		return fmt.Errorf("failed to write profiling data to file: %w", err)
	}
	return nil
}
