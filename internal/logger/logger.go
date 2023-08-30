package logger

import (
	"golang.org/x/exp/slog"
)

type (
	Logger interface {
		Debug(msg string, tags ...any)
		Info(msg string, tags ...any)
		Warn(msg string, tags ...any)
		Error(msg string, tags ...any)
	}
)

type slogLogger struct {
	logger *slog.Logger
}

func NewSlogLogger() Logger {
	return &slogLogger{
		logger: slog.Default(),
	}
}

func (sl *slogLogger) Debug(msg string, tags ...any) {
	sl.logger.Debug(msg, tags...)
}

func (sl *slogLogger) Info(msg string, tags ...any) {
	sl.logger.Info(msg, tags...)
}

func (sl *slogLogger) Warn(msg string, tags ...any) {
	sl.logger.Warn(msg, tags...)
}

func (sl *slogLogger) Error(msg string, tags ...any) {
	sl.logger.Error(msg, tags...)
}
