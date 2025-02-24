package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath" // Uses OS-specific separators (backslash on Windows, slash on Unix)
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
	"github.com/spf13/viper"
)

// Command holds the configuration for a command.
type Command struct {
	cmd   *cobra.Command
	run   func(cmd *Command, args []string) error
	flags []commandLineFlag
	cfg   *config.Config
	ctx   context.Context
	quiet bool
}

// NewCommand creates a new command instance with the given cobra command and run function.
func NewCommand(cmd *cobra.Command, flags []commandLineFlag, run func(cmd *Command, args []string) error) *cobra.Command {
	initFlags(cmd, flags...)

	command := &Command{flags: flags, run: run}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := command.init(cmd); err != nil {
			fmt.Printf("Initialization error: %v\n", err)
			os.Exit(1)
		}
		if err := command.run(command, args); err != nil {
			logger.Error(command.ctx, "Command failed", "err", err)
			os.Exit(1)
		}
		return nil
	}

	return cmd
}

// init initializes the application setup by loading configuration,
// setting up logger context, and logging any warnings.
func (c *Command) init(cmd *cobra.Command) error {
	ctx := cmd.Context()

	bindFlags(cmd, c.flags...)

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	var configLoaderOpts []config.ConfigLoaderOption

	// Use a custom config file if provided via the viper flag "config"
	if cfgPath := viper.GetString("config"); cfgPath != "" {
		configLoaderOpts = append(configLoaderOpts, config.WithConfigFile(cfgPath))
	}

	cfg, err := config.Load(configLoaderOpts...)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create a logger context based on config and quiet mode
	ctx = setupLoggerContext(cfg, ctx, quiet)

	// Log any warnings collected during configuration loading
	for _, w := range cfg.Warnings {
		logger.Warn(ctx, w)
	}

	c.cmd = cmd
	c.cfg = cfg
	c.ctx = ctx
	c.quiet = quiet
	return nil
}

// setupLoggerContext builds a logger context using options derived from configuration.
// It checks debug mode, quiet mode, and log format.
func setupLoggerContext(cfg *config.Config, ctx context.Context, quiet bool) context.Context {
	var opts []logger.Option
	if cfg.Global.Debug {
		opts = append(opts, logger.WithDebug())
	}
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if cfg.Global.LogFormat != "" {
		opts = append(opts, logger.WithFormat(cfg.Global.LogFormat))
	}
	return logger.WithLogger(ctx, logger.NewLogger(opts...))
}

// SetupWithConfig creates a setup instance from an existing configuration.
func SetupWithConfig(ctx context.Context, cfg *config.Config) *Command {
	return &Command{cfg: cfg, ctx: setupLoggerContext(cfg, ctx, false)}
}

// loggerContextWithFile returns a new logger context that writes logs to the given file.
// Useful when logs need to be persisted to a file.
func (cmd *Command) loggerContextWithFile(f *os.File) context.Context {
	var opts []logger.Option
	if cmd.quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if f != nil {
		opts = append(opts, logger.WithWriter(f))
	}
	return logger.WithLogger(cmd.ctx, logger.NewLogger(opts...))
}

// clientOption defines functional options for configuring the client.
type clientOption func(*clientOptions)

// clientOptions holds optional dependencies for constructing a client.
type clientOptions struct {
	dagStore     persistence.DAGStore
	historyStore persistence.HistoryStore
}

// withDAGStore returns a clientOption that sets a custom DAGStore.
func withDAGStore(dagStore persistence.DAGStore) clientOption {
	return func(o *clientOptions) {
		o.dagStore = dagStore
	}
}

// withHistoryStore returns a clientOption that sets a custom HistoryStore.
func withHistoryStore(historyStore persistence.HistoryStore) clientOption {
	return func(o *clientOptions) {
		o.historyStore = historyStore
	}
}

// Client initializes a Client using the provided options. If not supplied,
// it creates default DAGStore and HistoryStore instances.
func (s *Command) Client(opts ...clientOption) (client.Client, error) {
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
	// Create a flag store based on the suspend flags directory.
	flagStore := local.NewFlagStore(storage.NewStorage(
		s.cfg.Paths.SuspendFlagsDir,
	))

	return client.New(
		dagStore,
		historyStore,
		flagStore,
		s.cfg.Paths.Executable,
		s.cfg.Global.WorkDir,
	), nil
}

// server creates and returns a new web UI server.
// It initializes in-memory caches for DAGs and history, and uses them in the client.
func (s *Command) server(ctx context.Context) (*server.Server, error) {
	dagCache := filecache.New[*digraph.DAG](0, time.Hour*12)
	dagCache.StartEviction(ctx)
	dagStore := s.dagStoreWithCache(dagCache)

	historyCache := filecache.New[*model.Status](0, time.Hour*12)
	historyCache.StartEviction(ctx)
	historyStore := s.historyStoreWithCache(historyCache)

	cli, err := s.Client(withDAGStore(dagStore), withHistoryStore(historyStore))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}
	return frontend.New(s.cfg, cli), nil
}

// scheduler creates a new scheduler instance using the default client.
// It builds a DAG job manager to handle scheduled executions.
func (s *Command) scheduler() (*scheduler.Scheduler, error) {
	cli, err := s.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	manager := scheduler.NewDAGJobManager(s.cfg.Paths.DAGsDir, cli, s.cfg.Paths.Executable, s.cfg.Global.WorkDir)
	return scheduler.New(s.cfg, manager), nil
}

// dagStore returns a new DAGStore instance. It ensures that the directory exists
// (creating it if necessary) before returning the store.
func (s *Command) dagStore() (persistence.DAGStore, error) {
	baseDir := s.cfg.Paths.DAGsDir
	_, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to initialize directory %s: %w", baseDir, err)
		}
	}

	return local.NewDAGStore(s.cfg.Paths.DAGsDir), nil
}

// dagStoreWithCache returns a DAGStore instance that uses an in-memory file cache.
func (s *Command) dagStoreWithCache(cache *filecache.Cache[*digraph.DAG]) persistence.DAGStore {
	return local.NewDAGStore(s.cfg.Paths.DAGsDir, local.WithFileCache(cache))
}

// historyStore returns a new HistoryStore instance using JSON database storage.
// It applies the "latestStatusToday" setting from the server configuration.
func (s *Command) historyStore() persistence.HistoryStore {
	return jsondb.New(s.cfg.Paths.DataDir, jsondb.WithLatestStatusToday(
		s.cfg.Server.LatestStatusToday,
	))
}

// historyStoreWithCache returns a HistoryStore that uses an in-memory cache.
func (s *Command) historyStoreWithCache(cache *filecache.Cache[*model.Status]) persistence.HistoryStore {
	return jsondb.New(s.cfg.Paths.DataDir,
		jsondb.WithLatestStatusToday(s.cfg.Server.LatestStatusToday),
		jsondb.WithFileCache(cache),
	)
}

// OpenLogFile creates and opens a log file for a given DAG execution.
// It evaluates the log directory, validates settings, creates the log directory,
// builds a filename using the current timestamp and request ID, and then opens the file.
func (s *Command) OpenLogFile(
	ctx context.Context,
	prefix string,
	dag *digraph.DAG,
	requestID string,
) (*os.File, error) {
	logDir, err := cmdutil.EvalString(ctx, s.cfg.Paths.LogDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand log directory: %w", err)
	}

	config := LogFileSettings{
		Prefix:    prefix,
		LogDir:    logDir,
		DAGLogDir: dag.LogDir,
		DAGName:   dag.Name,
		RequestID: requestID,
	}

	if err := ValidateSettings(config); err != nil {
		return nil, fmt.Errorf("invalid log settings: %w", err)
	}

	outputDir, err := SetupLogDirectory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to setup log directory: %w", err)
	}

	filename := BuildLogFilename(config)
	return CreateLogFile(filepath.Join(outputDir, filename))
}

// generateRequestID creates a new UUID string to be used as a request identifier.
func generateRequestID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// signalListener is an interface for types that can receive OS signals.
type signalListener interface {
	Signal(context.Context, os.Signal)
}

// signalChan is a buffered channel to receive OS signals.
var signalChan = make(chan os.Signal, 100)

// listenSignals subscribes to SIGINT and SIGTERM signals and forwards them to the provided listener.
// It also listens for context cancellation and signals the listener with an os.Interrupt.
func listenSignals(ctx context.Context, listener signalListener) {
	go func() {
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		// If context is cancelled, signal with os.Interrupt.
		case <-ctx.Done():
			listener.Signal(ctx, os.Interrupt)
		// Forward the received signal.
		case sig := <-signalChan:
			listener.Signal(ctx, sig)
		}
	}()
}

// LogFileSettings defines configuration for log file creation.
type LogFileSettings struct {
	Prefix    string // Prefix for the log filename (e.g. "start_", "retry_").
	LogDir    string // Base directory for logs.
	DAGLogDir string // Optional alternative log directory specified by the DAG.
	DAGName   string // Name of the DAG; used for generating a safe directory name.
	RequestID string // Unique request ID used in the filename.
}

// ValidateSettings checks that essential fields are provided.
// It requires that DAGName is not empty and that at least one log directory is specified.
func ValidateSettings(config LogFileSettings) error {
	if config.DAGName == "" {
		return fmt.Errorf("DAGName cannot be empty")
	}
	if config.LogDir == "" && config.DAGLogDir == "" {
		return fmt.Errorf("either LogDir or DAGLogDir must be specified")
	}
	return nil
}

// SetupLogDirectory creates (if necessary) and returns the log directory based on the log file settings.
// It uses a safe version of the DAG name to avoid issues with invalid filesystem characters.
func SetupLogDirectory(config LogFileSettings) (string, error) {
	safeName := fileutil.SafeName(config.DAGName)

	// Choose the base directory: if DAGLogDir is provided, use it; otherwise use LogDir.
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

// BuildLogFilename constructs the log filename using the prefix, safe DAG name, current timestamp,
// and a truncated version of the request ID.
func BuildLogFilename(config LogFileSettings) string {
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

// CreateLogFile opens (or creates) the log file with flags for creation, write-only access,
// appending, and synchronous I/O. It sets file permissions to 0644.
func CreateLogFile(filepath string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND | os.O_SYNC
	permissions := os.FileMode(0644)

	file, err := os.OpenFile(filepath, flags, permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file %s: %w", filepath, err)
	}

	return file, nil
}
