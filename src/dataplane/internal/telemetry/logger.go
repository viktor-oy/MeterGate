package telemetry

import (
	"log/slog"
	"os"
)

// InitLogger configures the global slog logger to output JSON to stdout.
func InitLogger(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
