package logger

import (
	"context"
)

// WithLogger returns a new context with the given logger.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
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
	value := ctx.Value(contextKey{})
	if value == nil {
		defaultLogger.Warn("logger not found in the context")
		return defaultLogger
	}
	return value.(Logger)
}

// Debug logs a message with debug level.
func Debug(ctx context.Context, msg string, tags ...any) {
	FromContext(ctx).Debug(msg, tags...)
}

// Info logs a message with info level.
func Info(ctx context.Context, msg string, tags ...any) {
	FromContext(ctx).Info(msg, tags...)
}

// Warn logs a message with warn level.
func Warn(ctx context.Context, msg string, tags ...any) {
	FromContext(ctx).Warn(msg, tags...)
}

// Error logs a message with error level.
func Error(ctx context.Context, msg string, tags ...any) {
	FromContext(ctx).Error(msg, tags...)
}

// Fatal logs a message with fatal level and exits the program.
func Fatal(ctx context.Context, msg string, tags ...any) {
	FromContext(ctx).Fatal(msg, tags...)
}

// Debugf logs a formatted message with debug level.
func Debugf(ctx context.Context, format string, v ...any) {
	FromContext(ctx).Debugf(format, v...)
}

// Infof logs a formatted message with info level.
func Infof(ctx context.Context, format string, v ...any) {
	FromContext(ctx).Infof(format, v...)
}

// Warnf logs a formatted message with warn level.
func Warnf(ctx context.Context, format string, v ...any) {
	FromContext(ctx).Warnf(format, v...)
}

// Errorf logs a formatted message with error level.
func Errorf(ctx context.Context, format string, v ...any) {
	FromContext(ctx).Errorf(format, v...)
}

// Fatalf logs a formatted message with fatal level and exits the program.
func Fatalf(ctx context.Context, format string, v ...any) {
	FromContext(ctx).Fatalf(format, v...)
}

// Write writes a message with free form.
func Write(ctx context.Context, msg string) {
	FromContext(ctx).Write(msg)
}

type contextKey struct{}
type fixedKey struct{}
