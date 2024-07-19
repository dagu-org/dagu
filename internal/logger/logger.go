package logger

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/dagu-dev/dagu/internal/config"
)

type (
	Logger interface {
		Debug(msg string, tags ...any)
		Info(msg string, tags ...any)
		Warn(msg string, tags ...any)
		Error(msg string, tags ...any)

		Debugf(format string, v ...any)
		Infof(format string, v ...any)
		Warnf(format string, v ...any)
		Errorf(format string, v ...any)
	}
)

var _ Logger = (*appLogger)(nil)

type appLogger struct {
	logger *slog.Logger
}

// Debugf implements logger.Logger.
func (a *appLogger) Debugf(format string, v ...any) {
	a.logger.Debug(fmt.Sprintf(format, v...))
}

// Errorf implements logger.Logger.
func (a *appLogger) Errorf(format string, v ...any) {
	a.logger.Error(fmt.Sprintf(format, v...))
}

// Infof implements logger.Logger.
func (a *appLogger) Infof(format string, v ...any) {
	a.logger.Info(fmt.Sprintf(format, v...))
}

// Warnf implements logger.Logger.
func (a *appLogger) Warnf(format string, v ...any) {
	a.logger.Warn(fmt.Sprintf(format, v...))
}

func NewLogger(cfg *config.Config) Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{
		Level: level,
	}
	if level == slog.LevelDebug {
		opts.AddSource = true
	}
	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return &appLogger{
		logger: slog.New(handler),
	}
}

// Debug implements logger.Logger.
func (a *appLogger) Debug(msg string, tags ...any) {
	a.logger.Debug(msg, tags...)
}

// Error implements logger.Logger.
func (a *appLogger) Error(msg string, tags ...any) {
	a.logger.Error(msg, tags...)
}

// Info implements logger.Logger.
func (a *appLogger) Info(msg string, tags ...any) {
	a.logger.Info(msg, tags...)
}

// Warn implements logger.Logger.
func (a *appLogger) Warn(msg string, tags ...any) {
	a.logger.Warn(msg, tags...)
}
