// Package logger builds the process logger and offers thin helpers over it.
//
// New is called once in main; install its result with slog.SetDefault so the
// package-level helpers use the same handler and level.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Options configures the process logger.
type Options struct {
	// Level is debug, info, warn or error. An unknown value means info; the
	// configuration layer is responsible for rejecting typos.
	Level string
	// JSON selects slog's JSON handler, used in staging and production.
	JSON bool
	// AddSource records the source file and line on every record.
	AddSource bool
	// Writer receives the log output. Nil means os.Stdout.
	Writer io.Writer
}

// New builds the process logger. Call it once in main, then install it with
// slog.SetDefault so middleware, services and the helpers below share it.
func New(opts Options) *slog.Logger {
	writer := opts.Writer
	if writer == nil {
		writer = os.Stdout
	}

	handlerOpts := &slog.HandlerOptions{
		Level:     parseLevel(opts.Level),
		AddSource: opts.AddSource,
	}

	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(writer, handlerOpts)
	} else {
		handler = slog.NewTextHandler(writer, handlerOpts)
	}

	return slog.New(handler)
}

// parseLevel maps config.yaml log.level to slog. Unknown values fall back to
// info so a bad level never stops startup by itself.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Debug logs a message at DEBUG level on the default logger.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs a message at INFO level on the default logger.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs a message at WARN level — unexpected but recoverable situations.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs a message at ERROR level — operation failed, needs attention.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Fatal logs the message at ERROR level, then exits with status 1.
// Use only in main or startup code: os.Exit skips deferred cleanup, and a
// request handler must return its error instead.
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
