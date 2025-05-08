package commands

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"golang.hedera.com/solo-cheetah/internal/config"
	"golang.hedera.com/solo-cheetah/internal/core"
	"golang.hedera.com/solo-cheetah/internal/storage"
	"golang.hedera.com/solo-cheetah/pkg/logx"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload files to a remote storage",
	Long:  "Upload files to a remote storage",
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.ParseFlags(args); err != nil {
			logx.As().Error().Err(err).Msg("Failed to parse flags")
			os.Exit(1)
		}

		if cmd.Context() == nil {
			logx.As().Error().Msg("Context is nil")
			os.Exit(1)
		}

		runUpload(cmd.Context())
	},
}

func runUpload(ctx context.Context) {
	ctx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	// Initialize configuration
	if err := config.Initialize(flagConfig); err != nil {
		logx.As().Fatal().Err(err).Msg("Failed to initialize config")
		return
	}

	var wg sync.WaitGroup
	for _, pipeline := range config.Get().Pipelines {
		// Create scanner
		scanner, err := core.NewScanner(fmt.Sprintf("scanner-%s", pipeline.Name),
			pipeline.Scanner.Path, pipeline.Scanner.Pattern)
		if err != nil {
			logx.As().Error().Err(err).Msg("Failed to create scanner")
			return
		}

		// Prepare processors
		processors, err := prepareProcessors(pipeline)
		if err != nil {
			logx.As().Error().Err(err).Msg("Failed to prepare processors")
			return
		}

		// Start pipeline in a separate goroutine
		wg.Add(1)
		go func(p *config.PipelineConfig, s core.Scanner, ps []core.Processor) {
			defer wg.Done()
			err = startPipeline(ctx, p, s, ps)
			logx.As().Warn().Str("pipeline", p.Name).Msg("Pipeline stopped")
			if err != nil {
				logx.As().Error().Stack().Err(err).Msg("Stopping all pipelines because of error ")
				cancelFunc() // cancel all pipelines if one fails
			}
		}(pipeline, scanner, processors)
	}

	// wait for all pipelines to finish
	// we run in separate goroutine to avoid blocking the main thread that waits for OS signals to terminate
	go func() {
		wg.Wait()
		logx.As().Info().Msg("All pipelines are stopped")
	}()

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		logx.As().Info().Msg("Received exit signal, stopping pipelines...")
		cancelFunc()
	case <-ctx.Done():
	}

	time.Sleep(1 * time.Second)
}

func prepareProcessors(pc *config.PipelineConfig) ([]core.Processor, error) {
	// initialize processors
	var processors []core.Processor
	for i := 0; i < pc.Processor.MaxProcessors; i++ {
		var storages []core.Storage

		if pc.Processor.Storage.LocalDir.Enabled {
			localDir, err := storage.NewLocalDir(fmt.Sprintf("dir-%d-%s", i, pc.Name),
				*pc.Processor.Storage.LocalDir, *pc.Processor.Retry, pc.Processor.FileExtensions)
			if err != nil {
				return nil, fmt.Errorf("failed to create LocalDir storage: %w", err)
			}

			storages = append(storages, localDir)
		}

		if pc.Processor.Storage.RemoteHost.Enabled {
			remoteHost, err := storage.NewRemoteHost(fmt.Sprintf("host-%d-%s", i, pc.Name),
				*pc.Processor.Storage.RemoteHost, *pc.Processor.Retry)
			if err != nil {
				return nil, fmt.Errorf("failed to create RemoteHost storage: %w", err)
			}

			storages = append(storages, remoteHost)
		}

		if pc.Processor.Storage.S3.Enabled {
			s3, err := storage.NewS3(fmt.Sprintf("s3-%d-%s", i, pc.Name),
				*pc.Processor.Storage.S3, *pc.Processor.Retry, pc.Processor.FileExtensions)
			if err != nil {
				return nil, fmt.Errorf("failed to create S3 storage: %w", err)
			}

			storages = append(storages, s3)
		}

		if pc.Processor.Storage.GCS.Enabled {
			gcs, err := storage.NewGCSWithS3(fmt.Sprintf("gcs-%d-%s", i, pc.Name),
				*pc.Processor.Storage.GCS, *pc.Processor.Retry, pc.Processor.FileExtensions)
			if err != nil {
				return nil, fmt.Errorf("failed to create GCS storage: %w", err)
			}

			storages = append(storages, gcs)
		}

		p, err := core.NewProcessor(fmt.Sprintf("processor-%d-%s", i, pc.Name),
			storages, pc.Processor.FileExtensions)
		if err != nil {
			return nil, fmt.Errorf("failed to create processor: %w", err)
		}

		processors = append(processors, p)
	}

	return processors, nil
}

func startPipeline(ctx context.Context, c *config.PipelineConfig,
	scanner core.Scanner, processors []core.Processor) error {

	delay, err := time.ParseDuration(c.Scanner.Interval)
	if err != nil {
		return fmt.Errorf("error parsing watch interval: %w", err)
	}

	logx.As().Info().
		Str("pipeline", c.Name).
		Msg("Pipeline started")

	for {
		ech := make(chan error, 1) // Shared error channel for the pipeline, it is closed after all processors are done
		select {
		case <-ctx.Done():
			return nil
		default:
			var pwg sync.WaitGroup

			// Scan files
			items := scanner.Scan(ctx, c.Scanner.Path, ech)

			// Process files
			for _, processor := range processors {
				pwg.Add(1) // Add a wait group for each processor
				go func(p core.Processor) {
					defer pwg.Done() // Ensure the wait group is done when the processor finishes
					p.Process(ctx, items, ech)
					logx.As().Debug().
						Str("pipeline", c.Name).
						Str("processor", processor.Info()).
						Msg("Processor completed")
				}(processor)
			}

			// Wait for all processors to finish
			go func() {
				logx.As().Debug().
					Str("pipeline", c.Name).
					Msg("Waiting for processors to finish...")

				pwg.Wait() // Wait for all processors to complete

				logx.As().Debug().
					Str("pipeline", c.Name).
					Msg("All processors finished")

				close(ech) // Close the error channel after all processors are done
			}()

			errorFound := false
			for err := range ech {
				if err != nil {
					errorFound = true
					logx.As().Error().
						Str("pipeline", c.Name).
						Err(err).Msg("Error occurred in pipeline")
				}
			}

			if errorFound == true {
				return fmt.Errorf("pipeline '%s' encountered errors", c.Name)
			}

			// delay
			logx.As().Debug().
				Str("pipeline", c.Name).
				Dur("interval", delay).Msg("Sleeping before next scan...")
			time.Sleep(delay)
		}
	}
}
