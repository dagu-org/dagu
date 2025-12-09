package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/common/telemetry"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/persistence/filedag"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/persistence/fileproc"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/persistence/fileserviceregistry"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/frontend"
	"github.com/dagu-org/dagu/internal/service/resource"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Context holds the configuration for a command.
type Context struct {
	context.Context

	Command *cobra.Command
	Flags   []commandLineFlag
	Config  *config.Config
	Quiet   bool

	DAGRunStore     execution.DAGRunStore
	DAGRunMgr       runtime.Manager
	ProcStore       execution.ProcStore
	QueueStore      execution.QueueStore
	ServiceRegistry execution.ServiceRegistry

	Proc execution.ProcHandle
}

// LogToFile creates a new logger context with a file writer.
func (c *Context) LogToFile(f *os.File) {
	var opts []logger.Option
	if c.Config.Core.Debug {
		opts = append(opts, logger.WithDebug())
	}
	if c.Quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if c.Config.Core.LogFormat != "" {
		opts = append(opts, logger.WithFormat(c.Config.Core.LogFormat))
	}
	if f != nil {
		opts = append(opts, logger.WithWriter(f))
	}
	c.Context = logger.WithLogger(c.Context, logger.NewLogger(opts...))
}

// NewContext initializes the application setup by loading configuration,
// NewContext creates and initializes an application Context for the given Cobra command.
// It binds command flags, loads configuration scoped to the command, configures logging (respecting debug, quiet, and log format settings), logs any configuration warnings, and initializes history, DAG run, proc, queue, and service registry stores and managers used by the application.
// NewContext returns an initialized Context or an error if flag retrieval, configuration loading, or other initialization steps fail.
func NewContext(cmd *cobra.Command, flags []commandLineFlag) (*Context, error) {
	ctx := cmd.Context()

	v := viper.New()
	bindFlags(v, cmd, flags...)

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return nil, fmt.Errorf("failed to get quiet flag: %w", err)
	}
	daguHome, err := cmd.Flags().GetString("dagu-home")
	if err != nil {
		return nil, fmt.Errorf("failed to get dagu-home flag: %w", err)
	}

	var configLoaderOpts []config.ConfigLoaderOption
	if daguHome != "" {
		if resolvedHome := fileutil.ResolvePathOrBlank(daguHome); resolvedHome != "" {
			configLoaderOpts = append(configLoaderOpts, config.WithAppHomeDir(resolvedHome))
		}
	}

	// Use a custom config file if provided via the command flag "config"
	cfgPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, fmt.Errorf("failed to get config flag: %w", err)
	}
	if cfgPath != "" {
		configLoaderOpts = append(configLoaderOpts, config.WithConfigFile(cfgPath))
	}

	// Set service type based on command to load only necessary config sections
	configLoaderOpts = append(configLoaderOpts, config.WithService(serviceForCommand(cmd.Name())))

	loader := config.NewConfigLoader(v, configLoaderOpts...)
	cfg, err := loader.Load()
	if err != nil {
		return nil, err
	}
	ctx = config.WithConfig(ctx, cfg)

	// Create a logger context based on config and quiet mode
	var opts []logger.Option
	if cfg.Core.Debug || os.Getenv("DEBUG") != "" {
		opts = append(opts, logger.WithDebug())
	}
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if cfg.Core.LogFormat != "" {
		opts = append(opts, logger.WithFormat(cfg.Core.LogFormat))
	}
	ctx = logger.WithLogger(ctx, logger.NewLogger(opts...))

	// Log any warnings collected during configuration loading
	for _, w := range cfg.Warnings {
		logger.Warn(ctx, w)
	}

	// Initialize history repository and history manager
	hrOpts := []filedagrun.DAGRunStoreOption{
		filedagrun.WithLatestStatusToday(cfg.Server.LatestStatusToday),
		filedagrun.WithLocation(cfg.Core.Location),
	}

	switch cmd.Name() {
	case "server", "scheduler", "start-all":
		// For long-running process, we setup file cache for better performance
		hc := fileutil.NewCache[*execution.DAGRunStatus](0, time.Hour*12)
		hc.StartEviction(ctx)
		hrOpts = append(hrOpts, filedagrun.WithHistoryFileCache(hc))
	}

	ps := fileproc.New(cfg.Paths.ProcDir)
	drs := filedagrun.New(cfg.Paths.DAGRunsDir, hrOpts...)
	drm := runtime.NewManager(drs, ps, cfg)
	qs := filequeue.New(cfg.Paths.QueueDir)
	sm := fileserviceregistry.New(cfg.Paths.ServiceRegistryDir)

	// Log key configuration settings for debugging
	logger.Debug(ctx, "Configuration loaded",
		tag.Config(cfg.Paths.ConfigFileUsed),
		tag.Dir(cfg.Paths.DAGsDir),
	)
	logger.Debug(ctx, "Paths configuration",
		slog.String("log-dir", cfg.Paths.LogDir),
		slog.String("data-dir", cfg.Paths.DataDir),
		slog.String("dag-runs-dir", cfg.Paths.DAGRunsDir),
	)

	return &Context{
		Context:         ctx,
		Command:         cmd,
		Config:          cfg,
		Quiet:           quiet,
		DAGRunStore:     drs,
		DAGRunMgr:       drm,
		Flags:           flags,
		ProcStore:       ps,
		QueueStore:      qs,
		ServiceRegistry: sm,
	}, nil
}

// serviceForCommand returns the appropriate config.Service type for a given command name.
// serviceForCommand determines which config.Service to load for a given command name.
// "server" -> ServiceServer, "scheduler" -> ServiceScheduler, "worker" -> ServiceWorker,
// "coordinator" -> ServiceCoordinator, and "start", "restart", "retry", "dry", "exec" -> ServiceAgent.
// For any other command it returns ServiceNone so all configuration sections are loaded.
func serviceForCommand(cmdName string) config.Service {
	switch cmdName {
	case "server":
		return config.ServiceServer
	case "scheduler":
		return config.ServiceScheduler
	case "worker":
		return config.ServiceWorker
	case "coordinator":
		return config.ServiceCoordinator
	case "start", "restart", "retry", "dry", "exec":
		return config.ServiceAgent
	default:
		// For all other commands (status, stop, validate, etc.), load all config
		return config.ServiceNone
	}
}

// NewServer creates and returns a new web UI NewServer.
// It initializes in-memory caches for DAGs and runstore, and uses them in the client.
func (c *Context) NewServer(rs *resource.Service) (*frontend.Server, error) {
	dc := fileutil.NewCache[*core.DAG](0, time.Hour*12)
	dc.StartEviction(c)

	dr, err := c.dagStore(dc, nil)
	if err != nil {
		return nil, err
	}

	// Create coordinator client (may be nil if not configured)
	cc := c.NewCoordinatorClient()

	collector := telemetry.NewCollector(
		config.Version,
		dr,
		c.DAGRunStore,
		c.QueueStore,
		c.ServiceRegistry,
	)

	mr := telemetry.NewRegistry(collector)

	return frontend.NewServer(c.Config, dr, c.DAGRunStore, c.QueueStore, c.ProcStore, c.DAGRunMgr, cc, c.ServiceRegistry, mr, rs)
}

// NewCoordinatorClient creates a new coordinator client using the global peer configuration.
func (c *Context) NewCoordinatorClient() coordinator.Client {
	coordinatorCliCfg := coordinator.DefaultConfig()
	coordinatorCliCfg.CAFile = c.Config.Core.Peer.ClientCaFile
	coordinatorCliCfg.CertFile = c.Config.Core.Peer.CertFile
	coordinatorCliCfg.KeyFile = c.Config.Core.Peer.KeyFile
	coordinatorCliCfg.SkipTLSVerify = c.Config.Core.Peer.SkipTLSVerify
	coordinatorCliCfg.Insecure = c.Config.Core.Peer.Insecure

	if err := coordinatorCliCfg.Validate(); err != nil {
		logger.Error(c.Context, "Invalid coordinator client configuration", tag.Error(err))
		return nil
	}
	return coordinator.New(c.ServiceRegistry, coordinatorCliCfg)
}

// NewScheduler creates a new NewScheduler instance using the default client.
// It builds a DAG job manager to handle scheduled executions.
func (c *Context) NewScheduler() (*scheduler.Scheduler, error) {
	cache := fileutil.NewCache[*core.DAG](0, time.Hour*12)
	cache.StartEviction(c)

	dr, err := c.dagStore(cache, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DAG client: %w", err)
	}

	coordinatorCli := c.NewCoordinatorClient()
	de := scheduler.NewDAGExecutor(coordinatorCli, runtime.NewSubCmdBuilder(c.Config))
	m := scheduler.NewEntryReader(c.Config.Paths.DAGsDir, dr, c.DAGRunMgr, de, c.Config.Paths.Executable)
	return scheduler.New(c.Config, m, c.DAGRunMgr, c.DAGRunStore, c.QueueStore, c.ProcStore, c.ServiceRegistry, coordinatorCli)
}

// StringParam retrieves a string parameter from the command line flags.
// It checks if the parameter is wrapped in quotes and removes them if necessary.
func (c *Context) StringParam(name string) (string, error) {
	val, err := c.Command.Flags().GetString(name)
	if err != nil {
		return "", fmt.Errorf("failed to get flag %s: %w", name, err)
	}

	// If it's wrapped in quotes, remove them
	val = stringutil.RemoveQuotes(val)
	return val, nil
}

// dagStore returns a new DAGRepository instance. It ensures that the directory exists
// (creating it if necessary) before returning the store.
func (c *Context) dagStore(cache *fileutil.Cache[*core.DAG], searchPaths []string) (execution.DAGStore, error) {
	dir := c.Config.Paths.DAGsDir
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create DAGs directory %s: %w", dir, err)
		}
	}

	// Create a flag store based on the suspend flags directory.
	store := filedag.New(
		c.Config.Paths.DAGsDir,
		filedag.WithFlagsBaseDir(c.Config.Paths.SuspendFlagsDir),
		filedag.WithSearchPaths(searchPaths),
		filedag.WithFileCache(cache),
		filedag.WithSkipExamples(c.Config.Core.SkipExamples),
	)

	// Initialize the store (creates example DAGs if needed)
	if s, ok := store.(*filedag.Storage); ok {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize DAG store: %w", err)
		}
	}

	return store, nil
}

// OpenLogFile creates and opens a log file for a given dag-run.
// It evaluates the log directory, validates settings, creates the log directory,
// builds a filename using the current timestamp and dag-run ID, and then opens the file.
func (c *Context) OpenLogFile(
	dag *core.DAG,
	dagRunID string,
) (*os.File, error) {
	logPath, err := c.GenLogFileName(dag, dagRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate log file name: %w", err)
	}
	return fileutil.OpenOrCreateFile(logPath)
}

// GenLogFileName generates a log file name based on the DAG and dag-run ID.
func (c *Context) GenLogFileName(dag *core.DAG, dagRunID string) (string, error) {
	// Read the global configuration for log directory.
	baseLogDir, err := cmdutil.EvalString(c, c.Config.Paths.LogDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand log directory: %w", err)
	}

	// Read the log directory configuration from the DAG.
	dagLogDir, err := cmdutil.EvalString(c, dag.LogDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand DAG log directory: %w", err)
	}

	cfg := LogConfig{
		BaseDir:   baseLogDir,
		DAGLogDir: dagLogDir,
		Name:      dag.Name,
		DAGRunID:  dagRunID,
	}

	if err := cfg.Validate(); err != nil {
		return "", fmt.Errorf("invalid log settings: %w", err)
	}

	d, err := cfg.LogDir()
	if err != nil {
		return "", fmt.Errorf("failed to setup log directory: %w", err)
	}

	return filepath.Join(d, cfg.LogFile()), nil
}

// NewCommand creates a new command instance with the given cobra command and run function.
func NewCommand(cmd *cobra.Command, flags []commandLineFlag, runFunc func(cmd *Context, args []string) error) *cobra.Command {
	config.WithViperLock(func() {
		initFlags(cmd, flags...)
	})

	cmd.SilenceUsage = true

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Setup cpu profiling if enabled.
		cpuProfileEnabled, err := cmd.Flags().GetBool("cpu-profile")
		if err != nil {
			return fmt.Errorf("failed to read cpu-profile flag: %w", err)
		}
		if cpuProfileEnabled {
			f, err := os.Create("cpu.prof")
			if err != nil {
				return fmt.Errorf("failed to create CPU profile file: %w", err)
			}
			_ = pprof.StartCPUProfile(f)
			defer func() {
				pprof.StopCPUProfile()
				if err := f.Close(); err != nil {
					fmt.Printf("Failed to close CPU profile file: %v\n", err)
				}
			}()
		}

		ctx, err := NewContext(cmd, flags)

		if err != nil {
			return fmt.Errorf("initialization error: %w", err)
		}
		return runFunc(ctx, args)
	}

	return cmd
}

// genRunID creates a new UUID string to be used as a dag-run IDentifier.
func genRunID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// validateRunID checks if the dag-run ID is valid and not empty.
func validateRunID(dagRunID string) error {
	if dagRunID == "" {
		return ErrDAGRunIDRequired
	}
	if !reDAGRunID.MatchString(dagRunID) {
		return ErrDAGRunIDFormat
	}
	if len(dagRunID) > maxDAGRunIDLen {
		return ErrDAGRunIDTooLong
	}
	return nil
}

// reDAGRunID is a regular expression to validate dag-run IDs.
// It allows alphanumeric characters, hyphens, and underscores.
var reDAGRunID = regexp.MustCompile(`^[-a-zA-Z0-9_]+$`)

// maxDAGRunIDLen is the max length of the dag-run ID
const maxDAGRunIDLen = 64

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

// LogConfig defines configuration for log file creation.
type LogConfig struct {
	BaseDir   string // Base directory for logs.
	DAGLogDir string // Optional alternative log directory specified by the DAG definition.
	Name      string // Name of the DAG; used for generating a safe directory name.
	DAGRunID  string // Unique dag-run ID used in the filename.
}

// Validate checks that essential fields are provided.
// It requires that DAGName is not empty and that at least one log directory is specified.
func (cfg LogConfig) Validate() error {
	if cfg.Name == "" {
		return fmt.Errorf("DAGName cannot be empty")
	}
	if cfg.BaseDir == "" && cfg.DAGLogDir == "" {
		return fmt.Errorf("either LogDir or DAGLogDir must be specified")
	}
	return nil
}

// LogDir creates (if necessary) and returns the log directory based on the log file settings.
// It uses a safe version of the DAG name to avoid issues with invalid filesystem characters.
func (cfg LogConfig) LogDir() (string, error) {
	// Choose the base directory: if DAGLogDir is provided, use it; otherwise use LogDir.
	baseDir := cfg.BaseDir
	if cfg.DAGLogDir != "" {
		baseDir = cfg.DAGLogDir
	}
	if baseDir == "" {
		return "", fmt.Errorf("base log directory is not set")
	}

	utcTimestamp := time.Now().UTC().Format("20060102_150405Z")

	safeName := fileutil.SafeName(cfg.Name)
	logDir := filepath.Join(baseDir, safeName, "dag-run_"+utcTimestamp+"_"+cfg.DAGRunID)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return "", fmt.Errorf("failed to initialize directory %s: %w", logDir, err)
	}

	return logDir, nil
}

// LogFile constructs the log filename using the prefix, safe DAG name, current timestamp,
// and a truncated version of the dag-run ID.
func (cfg LogConfig) LogFile() string {
	timestamp := time.Now().Format("20060102.150405.000")
	truncDAGRunID := stringutil.TruncString(cfg.DAGRunID, 8)

	return fmt.Sprintf("dag-run_%s.%s.log",
		timestamp,
		truncDAGRunID,
	)
}