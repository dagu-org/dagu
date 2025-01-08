package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/dagu-org/dagu/internal/frontend/server"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func wrapRunE(f func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := f(cmd, args); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return nil
	}
}

type setup struct {
	cfg *config.Config
}

func newSetup(cfg *config.Config) *setup {
	return &setup{cfg: cfg}
}

func (s *setup) loggerContext(ctx context.Context, quiet bool) context.Context {
	var opts []logger.Option
	if s.cfg.Debug {
		opts = append(opts, logger.WithDebug())
	}
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if s.cfg.LogFormat != "" {
		opts = append(opts, logger.WithFormat(s.cfg.LogFormat))
	}
	return logger.WithLogger(ctx, logger.NewLogger(opts...))
}

func (s *setup) loggerContextWithFile(ctx context.Context, quiet bool, f *os.File) context.Context {
	var opts []logger.Option
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if f != nil {
		opts = append(opts, logger.WithWriter(f))
	}
	return logger.WithLogger(ctx, logger.NewLogger(opts...))
}

type clientOption func(*clientOptions)

type clientOptions struct {
	dagStore     persistence.DAGStore
	historyStore persistence.HistoryStore
}

func withDAGStore(dagStore persistence.DAGStore) clientOption {
	return func(o *clientOptions) {
		o.dagStore = dagStore
	}
}

func withHistoryStore(historyStore persistence.HistoryStore) clientOption {
	return func(o *clientOptions) {
		o.historyStore = historyStore
	}
}

func (s *setup) client(opts ...clientOption) (client.Client, error) {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}
	dagStore := options.dagStore
	if dagStore == nil {
		var err error
		dagStore, err = s.dagStore()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize DAG store: %w", err)
		}
	}
	historyStore := options.historyStore
	if historyStore == nil {
		historyStore = s.historyStore()
	}
	flagStore := local.NewFlagStore(storage.NewStorage(
		s.cfg.Paths.SuspendFlagsDir,
	))

	return client.New(
		dagStore,
		historyStore,
		flagStore,
		s.cfg.Paths.Executable,
		s.cfg.WorkDir,
	), nil
}

func (s *setup) server(ctx context.Context) (*server.Server, error) {
	dagCache := filecache.New[*digraph.DAG](0, time.Hour*12)
	dagCache.StartEviction(ctx)
	dagStore := s.dagStoreWithCache(dagCache)

	historyCache := filecache.New[*model.Status](0, time.Hour*12)
	historyCache.StartEviction(ctx)
	historyStore := s.historyStoreWithCache(historyCache)

	cli, err := s.client(withDAGStore(dagStore), withHistoryStore(historyStore))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}
	return frontend.New(s.cfg, cli), nil
}

func (s *setup) scheduler() (*scheduler.Scheduler, error) {
	cli, err := s.client()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}
	return scheduler.New(s.cfg, cli), nil
}

func (s *setup) dagStore() (persistence.DAGStore, error) {
	baseDir := s.cfg.Paths.DAGsDir
	_, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to initialize directory %s: %w", baseDir, err)
		}
	}

	return local.NewDAGStore(s.cfg.Paths.DAGsDir), nil
}

func (s *setup) dagStoreWithCache(cache *filecache.Cache[*digraph.DAG]) persistence.DAGStore {
	return local.NewDAGStore(s.cfg.Paths.DAGsDir, local.WithFileCache(cache))
}

func (s *setup) historyStore() persistence.HistoryStore {
	return jsondb.New(s.cfg.Paths.DataDir, jsondb.WithLatestStatusToday(
		s.cfg.LatestStatusToday,
	))
}

func (s *setup) historyStoreWithCache(cache *filecache.Cache[*model.Status]) persistence.HistoryStore {
	return jsondb.New(s.cfg.Paths.DataDir,
		jsondb.WithLatestStatusToday(s.cfg.LatestStatusToday),
		jsondb.WithFileCache(cache),
	)
}

func (s *setup) openLogFile(
	ctx context.Context,
	prefix string,
	dag *digraph.DAG,
	requestID string,
) (*os.File, error) {
	logDir, err := cmdutil.EvalString(ctx, s.cfg.Paths.LogDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand log directory: %w", err)
	}

	config := logFileSettings{
		Prefix:    prefix,
		LogDir:    logDir,
		DAGLogDir: dag.LogDir,
		DAGName:   dag.Name,
		RequestID: requestID,
	}

	if err := validateSettings(config); err != nil {
		return nil, fmt.Errorf("invalid log settings: %w", err)
	}

	outputDir, err := setupLogDirectory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to setup log directory: %w", err)
	}

	filename := buildLogFilename(config)
	return createLogFile(filepath.Join(outputDir, filename))
}

// generateRequestID generates a new request ID.
// For simplicity, we use UUIDs as request IDs.
func generateRequestID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

type signalListener interface {
	Signal(context.Context, os.Signal)
}

var signalChan = make(chan os.Signal, 100)

// listenSignals subscribes to the OS signals and passes them to the listener.
// It listens for the context cancellation as well.
func listenSignals(ctx context.Context, listener signalListener) {
	go func() {
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			listener.Signal(ctx, os.Interrupt)
		case sig := <-signalChan:
			listener.Signal(ctx, sig)
		}
	}()
}

// logFileSettings contains the settings for the log file.
type logFileSettings struct {
	Prefix    string
	LogDir    string
	DAGLogDir string
	DAGName   string
	RequestID string
}

// validateSettings ensures all required fields are properly set
func validateSettings(config logFileSettings) error {
	if config.DAGName == "" {
		return fmt.Errorf("DAGName cannot be empty")
	}
	if config.LogDir == "" && config.DAGLogDir == "" {
		return fmt.Errorf("either LogDir or DAGLogDir must be specified")
	}
	return nil
}

// setupLogDirectory creates and returns the appropriate log directory
func setupLogDirectory(config logFileSettings) (string, error) {
	safeName := fileutil.SafeName(config.DAGName)

	// Determine the base directory
	baseDir := config.LogDir
	if config.DAGLogDir != "" {
		baseDir = config.DAGLogDir
	}

	logDir := filepath.Join(baseDir, safeName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to initialize directory %s: %w", logDir, err)
	}

	return logDir, nil
}

// buildLogFilename generates the log filename using the configured format
func buildLogFilename(config logFileSettings) string {
	timestamp := time.Now().Format("20060102.15:04:05.000")
	truncatedRequestID := stringutil.TruncString(config.RequestID, 8)
	safeDagName := fileutil.SafeName(config.DAGName)

	return fmt.Sprintf("%s%s.%s.%s.log",
		config.Prefix,
		safeDagName,
		timestamp,
		truncatedRequestID,
	)
}

// createLogFile opens or creates a log file with appropriate permissions
func createLogFile(filepath string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND | os.O_SYNC
	permissions := os.FileMode(0644)

	file, err := os.OpenFile(filepath, flags, permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file %s: %w", filepath, err)
	}

	return file, nil
}
