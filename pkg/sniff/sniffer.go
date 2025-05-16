package sniff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.hedera.com/solo-cheetah/pkg/fsx"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"net/http"
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
	done             chan struct{}
	lastSnapshot     *Stats
}

func New(opts ProfilingConfig) *Sniffer {
	return &Sniffer{
		opts:             &opts,
		nextRotationHour: nil,
		done:             make(chan struct{}),
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
	if !s.opts.Enabled {
		return nil
	}

	now := time.Now()
	if s.nextRotationHour == nil {
		nextRotation := now.Truncate(time.Hour).Add(time.Hour)
		s.nextRotationHour = &nextRotation
	}

	if s.opts.EnableServer {
		if err := s.startServer(); err != nil {
			return err
		}
	}

	go func() {
		<-ctx.Done()
		logx.As().Debug().Msg("Context canceled, stopping stats capture...")
		s.Stop()
	}()

	return s.captureStats()
}

func (s *Sniffer) Stop() {
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
}

func (s *Sniffer) startServer() error {
	serverURL := fmt.Sprintf("%s:%d", s.opts.ServerHost, s.opts.ServerPort)
	server := &http.Server{Addr: serverURL}

	http.HandleFunc("/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		if s.lastSnapshot == nil {
			http.Error(w, "Stats not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(s.lastSnapshot); err != nil {
			http.Error(w, "Failed to encode stats", http.StatusInternalServerError)
		}
	})

	go func() {
		logx.As().Info().Msg(fmt.Sprintf("Starting pprof server on %s", serverURL))
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logx.As().Error().Err(err).Msg("pprof server failed")
		}
	}()

	go func(sv *http.Server) {
		<-s.done
		logx.As().Info().Msg("Shutting down pprof server...")
		if err := sv.Shutdown(context.Background()); err != nil {
			logx.As().Error().Err(err).Msg("Failed to shut down pprof server")
		}
	}(server)

	return nil
}

func (s *Sniffer) captureStats() error {
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
			case <-s.done:
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
					Msg("Captured runtime stats")
			}
		}
	}()

	logx.As().Info().Str("interval", s.opts.Interval).Str("file", statsFile).Msg("Started runtime stats capture")
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
	s.lastSnapshot = &Stats{
		Pid:       logx.GetPid(),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		MemStats:  memStats,
		CPUStats:  cpuStats,
	}

	if err := encoder.Encode(s.lastSnapshot); err != nil {
		return fmt.Errorf("failed to write stats to file: %w", err)
	}
	return nil
}
