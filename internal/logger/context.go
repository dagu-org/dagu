package logger

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"
)

// WithLogger returns a new context with the given logger.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// WithFixedLogger returns a new context with the given fixed logger.
// This is only used for testing.
func WithFixedLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, fixedKey{}, logger)
}

// FromContext returns a logger from the given context.
func FromContext(ctx context.Context) Logger {
	if value := ctx.Value(fixedKey{}); value != nil {
		return value.(Logger)
	}
	value := ctx.Value(loggerKey{})
	if value == nil {
		return defaultLogger
	}
	return value.(Logger)
}

// Debug logs a message with debug level.
func Debug(ctx context.Context, msg string, tags ...any) {
	logWithContextPC(ctx, slog.LevelDebug, msg, tags...)
}

// Info logs a message with info level.
func Info(ctx context.Context, msg string, tags ...any) {
	logWithContextPC(ctx, slog.LevelInfo, msg, tags...)
}

// Warn logs a message with warn level.
func Warn(ctx context.Context, msg string, tags ...any) {
	logWithContextPC(ctx, slog.LevelWarn, msg, tags...)
}

// Error logs a message with error level.
func Error(ctx context.Context, msg string, tags ...any) {
	logWithContextPC(ctx, slog.LevelError, msg, tags...)
}

// Fatal logs a message with fatal level and exits the program.
func Fatal(ctx context.Context, msg string, tags ...any) {
	logWithContextPC(ctx, slog.LevelError, msg, tags...)
}

// Debugf logs a formatted message with debug level.
func Debugf(ctx context.Context, format string, v ...any) {
	logWithContextPC(ctx, slog.LevelDebug, fmt.Sprintf(format, v...))
}

// Infof logs a formatted message with info level.
func Infof(ctx context.Context, format string, v ...any) {
	logWithContextPC(ctx, slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Warnf logs a formatted message with warn level.
func Warnf(ctx context.Context, format string, v ...any) {
	logWithContextPC(ctx, slog.LevelWarn, fmt.Sprintf(format, v...))
}

// Errorf logs a formatted message with error level.
func Errorf(ctx context.Context, format string, v ...any) {
	logWithContextPC(ctx, slog.LevelError, fmt.Sprintf(format, v...))
}

// Fatalf logs a formatted message with fatal level and exits the program.
func Fatalf(ctx context.Context, format string, v ...any) {
	logWithContextPC(ctx, slog.LevelError, fmt.Sprintf(format, v...))
}

// Write writes a message with free form.
func Write(ctx context.Context, msg string) {
	FromContext(ctx).Write(msg)
}

// logWithContextPC logs with the correct program counter, skipping context.go
func logWithContextPC(ctx context.Context, level slog.Level, msg string, tags ...any) {
	logger := FromContext(ctx)

	// Check if this is an appLogger with debug mode
	if appLog, ok := logger.(*appLogger); ok && appLog.debug {
		if !appLog.logger.Enabled(ctx, level) {
			return
		}

		// Get the caller's PC (skip runtime.Callers, this function, and the context function)
		var pcs [1]uintptr
		runtime.Callers(3, pcs[:])

		// Create record with correct PC
		record := slog.NewRecord(time.Now(), level, msg, pcs[0])
		if len(tags) > 0 {
			record.Add(tags...)
		}

		_ = appLog.logger.Handler().Handle(ctx, record)
		return
	}

	// Fallback to regular logging for non-appLogger or non-debug mode
	switch level {
	case slog.LevelDebug:
		logger.Debug(msg, tags...)
	case slog.LevelInfo:
		logger.Info(msg, tags...)
	case slog.LevelWarn:
		logger.Warn(msg, tags...)
	case slog.LevelError:
		logger.Error(msg, tags...)
	}
}

type loggerKey struct{}
type fixedKey struct{}
