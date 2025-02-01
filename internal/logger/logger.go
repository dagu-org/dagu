package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	slogmulti "github.com/samber/slog-multi"
)

type Logger interface {
	Debug(msg string, tags ...any)
	Info(msg string, tags ...any)
	Warn(msg string, tags ...any)
	Error(msg string, tags ...any)
	Fatal(msg string, tags ...any)

	Debugf(format string, v ...any)
	Infof(format string, v ...any)
	Warnf(format string, v ...any)
	Errorf(format string, v ...any)
	Fatalf(format string, v ...any)

	With(attrs ...any) Logger
	WithGroup(name string) Logger

	// Write writes a message to the logger in free form.
	Write(string)
}

var _ Logger = (*appLogger)(nil)

type appLogger struct {
	logger         *slog.Logger
	guardedHandler *guardedHandler
	quiet          bool
}

type Config struct {
	debug  bool
	format string
	writer io.Writer
	quiet  bool
}

type Option func(*Config)

// WithDebug sets the level of the logger to debug.
func WithDebug() Option {
	return func(o *Config) {
		o.debug = true
	}
}

// WithFormat sets the format of the logger (text or json).
func WithFormat(format string) Option {
	return func(o *Config) {
		o.format = format
	}
}

// WithWriter sets the file to write logs to.
func WithWriter(w io.Writer) Option {
	return func(o *Config) {
		o.writer = w
	}
}

// WithQuiet suppresses output to stderr.
func WithQuiet() Option {
	return func(o *Config) {
		o.quiet = true
	}
}

var defaultLogger = NewLogger(WithFormat("text"))

func NewLogger(opts ...Option) Logger {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	var level slog.Level
	if cfg.debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	if level == slog.LevelDebug {
		handlerOpts.AddSource = true
	}

	var (
		handlers       []slog.Handler
		guardedHandler *guardedHandler
	)

	if !cfg.quiet {
		handlers = append(handlers, newHandler(os.Stderr, cfg.format, handlerOpts))
	}

	if cfg.writer != nil {
		handler := newHandler(cfg.writer, cfg.format, handlerOpts)
		guardedHandler = newGuardedHandler(handler, cfg.writer)
		handlers = append(handlers, guardedHandler)
	}

	return &appLogger{
		logger:         slog.New(slogmulti.Fanout(handlers...)),
		guardedHandler: guardedHandler,
		quiet:          cfg.quiet,
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
	writer  io.Writer
	mu      sync.Mutex
}

func newGuardedHandler(handler slog.Handler, writer io.Writer) *guardedHandler {
	return &guardedHandler{
		handler: handler,
		writer:  writer,
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
		writer:  s.writer,
		mu:      sync.Mutex{},
	}
}

// WithGroup implements slog.Handler.
func (s *guardedHandler) WithGroup(name string) slog.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &guardedHandler{
		handler: s.handler.WithGroup(name),
		writer:  s.writer,
		mu:      sync.Mutex{},
	}
}

func newHandler(w io.Writer, format string, opts *slog.HandlerOptions) slog.Handler {
	if format == "text" {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}

// Debugf implements logger.Logger.
func (a *appLogger) Debugf(format string, v ...any) {
	a.logger.Debug(fmt.Sprintf(format, v...))
}

// Errorf implements logger.Logger.
func (a *appLogger) Errorf(format string, v ...any) {
	a.logger.Error(fmt.Sprintf(format, v...))
}

// Fatalf implements logger.Logger.
func (a *appLogger) Fatalf(format string, v ...any) {
	a.logger.Error(fmt.Sprintf(format, v...))
	os.Exit(1)
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

// Fatal implements logger.Logger.
func (a *appLogger) Fatal(msg string, tags ...any) {
	a.logger.Error(msg, tags...)
	os.Exit(1)
}

// Info implements logger.Logger.
func (a *appLogger) Info(msg string, tags ...any) {
	a.logger.Info(msg, tags...)
}

// Warn implements logger.Logger.
func (a *appLogger) Warn(msg string, tags ...any) {
	a.logger.Warn(msg, tags...)
}

// With implements logger.Logger.
func (a *appLogger) With(attrs ...any) Logger {
	return &appLogger{
		logger:         a.logger.With(attrs...),
		guardedHandler: a.guardedHandler,
	}
}

// WithGroup implements logger.Logger.
func (a *appLogger) WithGroup(name string) Logger {
	return &appLogger{
		logger:         a.logger.WithGroup(name),
		guardedHandler: a.guardedHandler,
	}
}

func (a *appLogger) Write(msg string) {
	// write to the standard output
	if !a.quiet {
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", msg)
	}
	if a.guardedHandler != nil {
		// If a guarded handler is present, write to the file
		a.guardedHandler.mu.Lock()
		defer a.guardedHandler.mu.Unlock()
		_, _ = a.guardedHandler.writer.Write([]byte(msg + "\n"))
	}
}
