package observability

import (
	"io"
	"log/slog"
	"os"
)

type LoggerConfig struct {
	Service string
	Env     string
	Version string
	Output  io.Writer
}

func NewLogger(cfg LoggerConfig) *slog.Logger {
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler).With(
		slog.String("service", cfg.Service),
		slog.String("env", cfg.Env),
		slog.String("version", cfg.Version),
	)
}
