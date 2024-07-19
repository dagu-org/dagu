package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/dagu-dev/dagu/internal/config"
	slogmulti "github.com/samber/slog-multi"
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

		// Write writes a free-form message to the logger.
		// It writes to the standard output and to the log file if present.
		// If the log file is not present, it writes only to the standard output.
		Write(string)
	}
)

var _ Logger = (*appLogger)(nil)

type appLogger struct {
	logger         *slog.Logger
	guardedHandler *guardedHandler
}

type NewLoggerArgs struct {
	Config  *config.Config
	LogFile *os.File
}

func NewLogger(args NewLoggerArgs) Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(args.Config.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{
		Level: level,
	}
	if level == slog.LevelDebug {
		opts.AddSource = true
	}

	var (
		handlers       []slog.Handler
		guardedHandler *guardedHandler
	)
	handlers = append(handlers, newHandler(os.Stderr, args.Config.LogFormat, opts))
	if args.LogFile != nil {
		guardedHandler = newGuardedHandler(
			newHandler(args.LogFile, args.Config.LogFormat, opts), args.LogFile,
		)
		handlers = append(handlers, guardedHandler)
	}
	return &appLogger{
		logger: slog.New(
			slogmulti.Fanout(handlers...),
		),
		guardedHandler: guardedHandler,
	}
}

var _ slog.Handler = (*guardedHandler)(nil)

// guardedHandler is a slog.Handler that guards writes to a file with a mutex.
// This is to allow appLogger to write to the same file without interleaving
// log lines. It assumes:
// 1. that the underlying handler is thread-safe and.
// 2. the file is opened with the O_SYNC flag to ensure that writes are atomic.
type guardedHandler struct {
	handler slog.Handler
	file    *os.File
	mu      sync.Mutex
}

func newGuardedHandler(handler slog.Handler, file *os.File) *guardedHandler {
	return &guardedHandler{
		handler: handler,
		file:    file,
	}
}

// Enabled implements slog.Handler.
func (s *guardedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return s.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (s *guardedHandler) Handle(ctx context.Context, record slog.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handler.Handle(ctx, record)
}

// WithAttrs implements slog.Handler.
func (s *guardedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &guardedHandler{
		handler: s.handler.WithAttrs(attrs),
		file:    s.file,
		mu:      sync.Mutex{},
	}
}

// WithGroup implements slog.Handler.
func (s *guardedHandler) WithGroup(name string) slog.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &guardedHandler{
		handler: s.handler.WithGroup(name),
		file:    s.file,
		mu:      sync.Mutex{},
	}
}

func newHandler(f *os.File, format string, opts *slog.HandlerOptions) slog.Handler {
	if format == "text" {
		return slog.NewTextHandler(f, opts)
	}
	return slog.NewJSONHandler(f, opts)
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

// Write implements logger.Logger.
func (a *appLogger) Write(msg string) {
	// write to the standard output
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", msg)
	// If a guarded handler is present, write to the file
	if a.guardedHandler == nil {
		return
	}
	a.guardedHandler.mu.Lock()
	defer a.guardedHandler.mu.Unlock()
	_, _ = a.guardedHandler.file.WriteString(msg)
}
