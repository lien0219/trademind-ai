package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures the global slog logger from APP_ENV.
// development → text handler; production → JSON to stdout.
func Init(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(appEnv, "development") || appEnv == "" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.EqualFold(appEnv, "production") {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	log := slog.New(h)
	slog.SetDefault(log)
	return log
}

// L is shorthand for the global default logger (after Init).
func L() *slog.Logger {
	return slog.Default()
}
