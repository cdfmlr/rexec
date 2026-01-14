package rexec

import (
	"io"
	"log/slog"
	"os"
)

// useDebugLogger enables the debug logger for rexec package.
const useDebugLogger = false

// This file defines a (slog) logger used by the rexec package.
// The log is disabled by default.

// Logger is the logger used by the rexec package.
//
// The logging is disabled by default by setting it to a null logger that
// discards all logs.
//
// Callers can assign a different logger to this variable to enable logging:
//
//	rexec.Logger = slog.Default().With("pkg", "rexec")
var Logger *slog.Logger

func init() {
	// Logger = slog.Default().With("pkg", "rexec")

	if useDebugLogger {
		Logger = debugLogger()
	} else {
		Logger = nullLogger()
	}
}

// nullLogger returns a logger that discards all logs.
func nullLogger() *slog.Logger {
	handler := slog.NewJSONHandler(
		io.Discard,
		&slog.HandlerOptions{
			Level: slog.LevelError + 1,
		})

	return slog.New(handler)
}

// debugLogger is a JSON logger at debug level that logs to stderr.
// It is used for debugging rexec package.
func debugLogger() *slog.Logger {
	handler := slog.NewJSONHandler(
		os.Stderr,
		&slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

	return slog.New(handler)
}
