package logging

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config describes logger runtime configuration.
type Config struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	TimeFormat  string `mapstructure:"time_format"`
	Caller      bool   `mapstructure:"caller"`
	PrettyPrint bool   `mapstructure:"pretty"`
}

// NewLogger constructs a zerolog logger from config.
func NewLogger(cfg Config) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	if cfg.TimeFormat != "" {
		zerolog.TimeFieldFormat = cfg.TimeFormat
	}

	level := zerolog.InfoLevel
	if parsed, err := zerolog.ParseLevel(strings.ToLower(cfg.Level)); err == nil {
		level = parsed
	}

	writer := logWriter(cfg)
	logger := zerolog.New(writer).Level(level)
	builder := logger.With().Timestamp()
	if cfg.Caller {
		builder = builder.Caller()
	}

	return builder.Logger()
}

func logWriter(cfg Config) io.Writer {
	if cfg.PrettyPrint || strings.EqualFold(cfg.Format, "console") {
		return zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: zerolog.TimeFieldFormat,
		}
	}
	return os.Stdout
}
