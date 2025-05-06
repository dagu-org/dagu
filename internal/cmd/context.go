package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath" // Uses OS-specific separators (backslash on Windows, slash on Unix)
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/dagstore"
	"github.com/dagu-org/dagu/internal/dagstore/filestore"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/runstore"
	runfs "github.com/dagu-org/dagu/internal/runstore/filestore"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var _ context.Context = (*Context)(nil)

// Context holds the configuration for a command.
type Context struct {
	*cobra.Command

	run   func(cmd *Context, args []string) error
	flags []commandLineFlag
	cfg   *config.Config
	ctx   context.Context
	quiet bool
}

// Deadline implements context.Context.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

// Done implements context.Context.
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err implements context.Context.
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value implements context.Context.
func (c *Context) Value(key any) any {
	return c.ctx.Value(key)
}

// LogToFile creates a new logger context with a file writer.
func (c *Context) LogToFile(f *os.File) {
	var opts []logger.Option
	if c.cfg.Global.Debug {
		opts = append(opts, logger.WithDebug())
	}
	if c.quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if c.cfg.Global.LogFormat != "" {
		opts = append(opts, logger.WithFormat(c.cfg.Global.LogFormat))
	}
	if f != nil {
		opts = append(opts, logger.WithWriter(f))
	}
	c.ctx = logger.WithLogger(c.ctx, logger.NewLogger(opts...))
}

// init initializes the application setup by loading configuration,
// setting up logger context, and logging any warnings.
func (c *Context) init(cmd *cobra.Command) error {
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
	opts := c.loggingOpts(cfg)
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	ctx = setupLoggerContext(ctx, opts...)

	// Log any warnings collected during configuration loading
	for _, w := range cfg.Warnings {
		logger.Warn(ctx, w)
	}

	c.Command = cmd
	c.cfg = cfg
	c.ctx = ctx
	c.quiet = quiet

	return nil
}

// Client initializes a Client using the provided options. If not supplied,
// it creates default DAGStore and RunStore instances.
func (s *Context) Client(opts ...clientOption) (runstore.Client, error) {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}
	runStore := options.runStore
	if runStore == nil {
		runStore = s.runStore()
	}

	return runstore.NewClient(
		runStore,
		s.cfg.Paths.Executable,
		s.cfg.Global.WorkDir,
	), nil
}

// DAGClient initializes a DAGClient using the provided options.
func (s *Context) DAGClient(runClient runstore.Client, opts ...dagClientOption) (dagstore.Client, error) {
	options := &dagClientOptions{}
	for _, opt := range opts {
		opt(options)
	}
	dagStore := options.dagStore
	if dagStore == nil {
		var err error
		dagStore, err = s.dagStore(nil)
		if err != nil {
			return dagstore.Client{}, fmt.Errorf("failed to initialize DAG store: %w", err)
		}
	}

	return dagstore.NewClient(
		runClient,
		dagStore,
	), nil
}

// server creates and returns a new web UI server.
// It initializes in-memory caches for DAGs and runstore, and uses them in the client.
func (ctx *Context) server() (*frontend.Server, error) {
	dagCache := fileutil.NewCache[*digraph.DAG](0, time.Hour*12)
	dagCache.StartEviction(ctx)
	dagStore := ctx.dagStoreWithCache(dagCache)

	statusCache := fileutil.NewCache[*runstore.Status](0, time.Hour*12)
	statusCache.StartEviction(ctx)
	runStore := ctx.runStoreWithCache(statusCache)

	runCli, err := ctx.Client(withRunStore(runStore))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	dagCli, err := ctx.DAGClient(runCli, withDAGStore(dagStore))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DAG client: %w", err)
	}

	return frontend.NewServer(ctx.cfg, dagCli, runCli), nil
}

// scheduler creates a new scheduler instance using the default client.
// It builds a DAG job manager to handle scheduled executions.
func (s *Context) scheduler() (*scheduler.Scheduler, error) {
	runCli, err := s.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	dagCli, err := s.DAGClient(runCli)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DAG client: %w", err)
	}

	manager := scheduler.NewDAGJobManager(s.cfg.Paths.DAGsDir, dagCli, runCli, s.cfg.Paths.Executable, s.cfg.Global.WorkDir)
	return scheduler.New(s.cfg, manager), nil
}

// dagStore returns a new DAGStore instance. It ensures that the directory exists
// (creating it if necessary) before returning the store.
func (s *Context) dagStore(searchPaths []string) (dagstore.Store, error) {
	baseDir := s.cfg.Paths.DAGsDir
	_, err := os.Stat(baseDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0750); err != nil {
			return nil, fmt.Errorf("failed to initialize directory %s: %w", baseDir, err)
		}
	}

	// Create a flag store based on the suspend flags directory.
	return filestore.New(
		s.cfg.Paths.DAGsDir,
		filestore.WithFlagsBaseDir(s.cfg.Paths.SuspendFlagsDir),
		filestore.WithSearchPaths(searchPaths)), nil
}

// dagStoreWithCache returns a DAGStore instance that uses an in-memory file cache.
func (s *Context) dagStoreWithCache(cache *fileutil.Cache[*digraph.DAG]) dagstore.Store {
	return filestore.New(s.cfg.Paths.DAGsDir, filestore.WithFlagsBaseDir(s.cfg.Paths.SuspendFlagsDir), filestore.WithFileCache(cache))
}

// runStore returns a new RunStore instance using JSON database storage.
// It applies the "latestStatusToday" setting from the server configuration.
func (s *Context) runStore() runstore.Store {
	return runfs.New(s.cfg.Paths.DataDir, runfs.WithLatestStatusToday(
		s.cfg.Server.LatestStatusToday,
	))
}

// runStoreWithCache returns a RunStore that uses an in-memory cache.
func (s *Context) runStoreWithCache(cache *fileutil.Cache[*runstore.Status]) runstore.Store {
	return runfs.New(s.cfg.Paths.DataDir,
		runfs.WithLatestStatusToday(s.cfg.Server.LatestStatusToday),
		runfs.WithFileCache(cache),
	)
}

// OpenLogFile creates and opens a log file for a given DAG run.
// It evaluates the log directory, validates settings, creates the log directory,
// builds a filename using the current timestamp and request ID, and then opens the file.
func (ctx *Context) OpenLogFile(
	dag *digraph.DAG,
	requestID string,
) (*os.File, error) {
	logDir, err := cmdutil.EvalString(ctx, ctx.cfg.Paths.LogDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand log directory: %w", err)
	}

	dagLogDir, err := cmdutil.EvalString(ctx, dag.LogDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand DAG log directory: %w", err)
	}

	config := LogFileSettings{
		LogDir:    logDir,
		DAGLogDir: dagLogDir,
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
	return OpenOrCreateLogFile(filepath.Join(outputDir, filename))
}

func (c *Context) loggingOpts(cfg *config.Config) []logger.Option {
	var opts []logger.Option
	if cfg.Global.Debug || os.Getenv("DEBUG") != "" {
		opts = append(opts, logger.WithDebug())
	}
	if c.quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if cfg.Global.LogFormat != "" {
		opts = append(opts, logger.WithFormat(cfg.Global.LogFormat))
	}
	return opts
}

// NewCommand creates a new command instance with the given cobra command and run function.
func NewCommand(cmd *cobra.Command, flags []commandLineFlag, run func(cmd *Context, args []string) error) *cobra.Command {
	initFlags(cmd, flags...)

	ctx := &Context{flags: flags, run: run}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := ctx.init(cmd); err != nil {
			fmt.Printf("Initialization error: %v\n", err)
			os.Exit(1)
		}
		if err := ctx.run(ctx, args); err != nil {
			logger.Error(ctx.ctx, "Command failed", "err", err)
			os.Exit(1)
		}
		return nil
	}

	return cmd
}

// setupLoggerContext builds a logger context using options derived from configuration.
// It checks debug mode, quiet mode, and log format.
func setupLoggerContext(ctx context.Context, opts ...logger.Option) context.Context {
	return logger.WithLogger(ctx, logger.NewLogger(opts...))
}

// NewContext creates a setup instance from an existing configuration.
func NewContext(ctx context.Context, cfg *config.Config) *Context {
	c := &Context{cfg: cfg}
	c.ctx = setupLoggerContext(ctx, c.loggingOpts(cfg)...)
	return c
}

// clientOption defines functional options for configuring the client.
type clientOption func(*clientOptions)

// clientOptions holds optional dependencies for constructing a client.
type clientOptions struct {
	runStore runstore.Store
}

// withRunStore returns a clientOption that sets a custom RunStore.
func withRunStore(historyStore runstore.Store) clientOption {
	return func(o *clientOptions) {
		o.runStore = historyStore
	}
}

// dagClientOption defines functional options for configuring the DAG client.
type dagClientOption func(*dagClientOptions)

// dagClientOption defines functional options for configuring the DAG client.
type dagClientOptions struct {
	dagStore dagstore.Store
}

// withDAGStore returns a clientOption that sets a custom DAGStore.
func withDAGStore(dagStore dagstore.Store) dagClientOption {
	return func(o *dagClientOptions) {
		o.dagStore = dagStore
	}
}

// generateRequestID creates a new UUID string to be used as a request identifier.
func generateRequestID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// validateRequestID checks if the request ID is valid and not empty.
func validateRequestID(requestID string) error {
	if requestID == "" {
		return fmt.Errorf("request ID is not set")
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}
	return nil
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
	if baseDir == "" {
		return "", fmt.Errorf("base log directory is not set")
	}

	logDir := filepath.Join(baseDir, safeName)
	if err := os.MkdirAll(logDir, 0750); err != nil {
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

	return fmt.Sprintf("scheduler_%s.%s.%s.log",
		safeDagName,
		timestamp,
		truncatedRequestID,
	)
}

// OpenOrCreateLogFile opens (or creates) the log file with flags for creation, write-only access,
// appending, and synchronous I/O. It sets file permissions to 0600.
func OpenOrCreateLogFile(filepath string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND | os.O_SYNC
	file, err := os.OpenFile(filepath, flags, 0600) // nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file %s: %w", filepath, err)
	}

	return file, nil
}
