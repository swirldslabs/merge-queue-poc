package logx

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"golang.hedera.com/solo-cheetah/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path"
	"time"
)

var logger zerolog.Logger
var startTime time.Time
var pid = os.Getpid()

func Initialize() error {
	return InitializeWithOptions(config.Get().Log)
}

func newRollingFile(cfg *config.LoggingConfig) (io.Writer, error) {
	return &lumberjack.Logger{
		Filename:   path.Join(cfg.Directory, cfg.Filename),
		MaxBackups: cfg.MaxBackups, // files
		MaxSize:    cfg.MaxSize,    // megabytes
		MaxAge:     cfg.MaxAge,     // days
		Compress:   cfg.Compress,
	}, nil
}

func InitializeWithOptions(cfg *config.LoggingConfig) error {
	l, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(l)
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	console := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	var writers []io.Writer
	if cfg.FileLogging {
		logFile, err := newRollingFile(cfg)
		if err != nil {
			return err
		}

		fileWriter := zerolog.New(logFile).With().Timestamp().Logger()
		writers = append(writers, console, fileWriter)
	} else {
		writers = append(writers, console)
	}

	mw := zerolog.MultiLevelWriter(writers...)
	logger = zerolog.New(mw).With().
		Timestamp().
		Int("pid", pid).
		Logger()

	return nil
}

func As() *zerolog.Logger {
	return &logger
}

func StartTimer() {
	startTime = time.Now()
}

func ExecutionTime() string {
	return time.Since(startTime).Round(time.Second).String()
}

func GetPid() int {
	return pid
}
