package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/cmn/telemetry"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedag"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/persis/filenamespace"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/dagu-org/dagu/internal/persis/filequeue"
	"github.com/dagu-org/dagu/internal/persis/fileserviceregistry"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/transform"
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

	DAGRunStore     exec.DAGRunStore
	DAGRunMgr       runtime.Manager
	ProcStore       exec.ProcStore
	QueueStore      exec.QueueStore
	ServiceRegistry exec.ServiceRegistry
	NamespaceStore  exec.NamespaceStore

	Proc             exec.ProcHandle
	NamespaceShortID string // Set by ResolveNamespace/ResolveNamespaceFromArg
}

// WithContext returns a new Context with a different underlying context.Context.
// This is useful for creating a signal-aware context for service operations.
func (c *Context) WithContext(ctx context.Context) *Context {
	return &Context{
		Context:          ctx,
		Command:          c.Command,
		Flags:            c.Flags,
		Config:           c.Config,
		Quiet:            c.Quiet,
		DAGRunStore:      c.DAGRunStore,
		DAGRunMgr:        c.DAGRunMgr,
		ProcStore:        c.ProcStore,
		QueueStore:       c.QueueStore,
		ServiceRegistry:  c.ServiceRegistry,
		NamespaceStore:   c.NamespaceStore,
		Proc:             c.Proc,
		NamespaceShortID: c.NamespaceShortID,
	}
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

// NewContext creates and initializes an application Context for the given Cobra command.
// It binds command flags, loads configuration scoped to the command, configures logging
// (respecting debug, quiet, and log format settings), logs any configuration warnings,
// and initializes history, DAG run, proc, queue, and service registry stores and managers.
// Returns an initialized Context or an error if flag retrieval, configuration loading,
// or other initialization steps fail.
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
	// For agent commands running in a terminal, suppress console output early
	// to avoid debug logs cluttering the progress display or tree output
	if !quiet && isAgentCommand(cmd.Name()) && term.IsTerminal(int(os.Stderr.Fd())) && os.Getenv("DISABLE_PROGRESS") == "" {
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

	// For shared-nothing workers, skip creating file-based stores
	// as they only use temporary directories and push status to coordinator
	if isSharedNothingWorker(cmd, cfg) {
		logger.Debug(ctx, "Shared-nothing worker mode: skipping file-based stores",
			slog.Any("coordinators", cfg.Worker.Coordinators),
		)
		return &Context{
			Context: ctx,
			Command: cmd,
			Config:  cfg,
			Quiet:   quiet,
			Flags:   flags,
			// All stores are nil - shared-nothing workers don't need local storage
			// Status is pushed to coordinator, DAG definitions come from task payload
		}, nil
	}

	// Initialize history repository and history manager
	hrOpts := []filedagrun.DAGRunStoreOption{
		filedagrun.WithLatestStatusToday(cfg.Server.LatestStatusToday),
		filedagrun.WithLocation(cfg.Core.Location),
	}

	switch cmd.Name() {
	case "server", "scheduler", "start-all", "coordinator":
		// For long-running process, we setup file cache for better performance
		limits := cfg.Cache.Limits()
		hc := fileutil.NewCache[*exec.DAGRunStatus]("dag_run_status", limits.DAGRun.Limit, limits.DAGRun.TTL)
		hc.StartEviction(ctx)
		hrOpts = append(hrOpts, filedagrun.WithHistoryFileCache(hc))
	}

	ps := fileproc.New(cfg.Paths.ProcDir)
	drs := filedagrun.New(cfg.Paths.DAGRunsDir, hrOpts...)
	drm := runtime.NewManager(drs, ps, cfg)
	qs := filequeue.New(cfg.Paths.QueueDir)
	sm := fileserviceregistry.New(cfg.Paths.ServiceRegistryDir)
	ns, err := filenamespace.New(cfg.Paths.NamespacesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize namespace store: %w", err)
	}

	// Auto-migrate existing data into the default namespace on first startup.
	if err := migrateToDefaultNamespace(cfg.Paths); err != nil {
		return nil, fmt.Errorf("failed to run namespace migration: %w", err)
	}

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
		NamespaceStore:  ns,
	}, nil
}

// serviceForCommand determines which config.Service to load for a given command name.
// Returns the appropriate service type for the command, or ServiceNone to load all config.
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

// isAgentCommand returns true if the command name is an agent command
// that displays progress or tree output.
func isAgentCommand(cmdName string) bool {
	switch cmdName {
	case "start", "restart", "retry", "dry", "exec":
		return true
	default:
		return false
	}
}

// isSharedNothingWorker checks if the current command is a worker with static coordinators
// configured, indicating shared-nothing mode where no local storage is needed.
func isSharedNothingWorker(cmd *cobra.Command, cfg *config.Config) bool {
	if cmd.Name() != "worker" {
		return false
	}
	return len(cfg.Worker.Coordinators) > 0
}

// NewServer creates and returns a new web UI NewServer.
// It initializes in-memory caches for DAGs and runstore, and uses them in the client.
func (c *Context) NewServer(rs *resource.Service, opts ...frontend.ServerOption) (*frontend.Server, error) {
	limits := c.Config.Cache.Limits()
	dc := fileutil.NewCache[*core.DAG]("dag_definition", limits.DAG.Limit, limits.DAG.TTL)
	dc.StartEviction(c)

	dr, err := c.dagStore(dagStoreConfig{Cache: dc})
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

	// Register DAG definition cache for metrics
	collector.RegisterCache(dc)

	mr := telemetry.NewRegistry(collector)

	if c.NamespaceStore != nil {
		opts = append(opts, frontend.WithNamespaceStore(c.NamespaceStore))
	}
	return frontend.NewServer(c.Context, c.Config, dr, c.DAGRunStore, c.QueueStore, c.ProcStore, c.DAGRunMgr, cc, c.ServiceRegistry, mr, collector, rs, opts...)
}

// NewCoordinatorClient creates a new coordinator client using the global peer configuration.
func (c *Context) NewCoordinatorClient() coordinator.Client {
	coordinatorCliCfg := coordinator.DefaultConfig()
	coordinatorCliCfg.CAFile = c.Config.Core.Peer.ClientCaFile
	coordinatorCliCfg.CertFile = c.Config.Core.Peer.CertFile
	coordinatorCliCfg.KeyFile = c.Config.Core.Peer.KeyFile
	coordinatorCliCfg.SkipTLSVerify = c.Config.Core.Peer.SkipTLSVerify
	coordinatorCliCfg.Insecure = c.Config.Core.Peer.Insecure

	// Use config values for retry if provided
	if c.Config.Core.Peer.MaxRetries > 0 {
		coordinatorCliCfg.MaxRetries = c.Config.Core.Peer.MaxRetries
	}
	if c.Config.Core.Peer.RetryInterval > 0 {
		coordinatorCliCfg.RetryInterval = c.Config.Core.Peer.RetryInterval
	}

	if err := coordinatorCliCfg.Validate(); err != nil {
		logger.Error(c.Context, "Invalid coordinator client configuration", tag.Error(err))
		return nil
	}
	return coordinator.New(c.ServiceRegistry, coordinatorCliCfg)
}

// NewScheduler creates a new NewScheduler instance using the default client.
// It builds a DAG job manager to handle scheduled executions.
func (c *Context) NewScheduler() (*scheduler.Scheduler, error) {
	limits := c.Config.Cache.Limits()
	cache := fileutil.NewCache[*core.DAG]("dag_definition", limits.DAG.Limit, limits.DAG.TTL)
	cache.StartEviction(c)

	dr, err := c.dagStore(dagStoreConfig{Cache: cache})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DAG client: %w", err)
	}

	coordinatorCli := c.NewCoordinatorClient()
	de := scheduler.NewDAGExecutor(coordinatorCli, runtime.NewSubCmdBuilder(c.Config))
	m := scheduler.NewEntryReader(c.Config.Paths.DAGsDir, dr, c.DAGRunMgr, de, c.Config.Paths.Executable, c.NamespaceStore)
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

// getWorkerID retrieves the worker ID from context, defaulting to "local" if not set or on error.
func getWorkerID(ctx *Context) string {
	workerID, err := ctx.StringParam("worker-id")
	if err != nil {
		logger.Warn(ctx, "Failed to read worker-id flag, defaulting to 'local'", tag.Error(err))
		return "local"
	}
	if workerID == "" {
		return "local"
	}
	return workerID
}

// dagStoreConfig contains options for creating a DAG store.
type dagStoreConfig struct {
	Cache                 *fileutil.Cache[*core.DAG] // Optional cache for DAG objects
	SearchPaths           []string                   // Additional search paths for DAG files
	SkipDirectoryCreation bool                       // Skip directory creation (for distributed worker execution)
}

// dagStore returns a new DAGRepository instance.
func (c *Context) dagStore(cfg dagStoreConfig) (exec.DAGStore, error) {
	store := filedag.New(
		c.Config.Paths.DAGsDir,
		filedag.WithFlagsBaseDir(c.Config.Paths.SuspendFlagsDir),
		filedag.WithSearchPaths(cfg.SearchPaths),
		filedag.WithFileCache(cfg.Cache),
		filedag.WithSkipExamples(c.Config.Core.SkipExamples),
		filedag.WithSkipDirectoryCreation(cfg.SkipDirectoryCreation),
	)

	// Initialize the store (creates directory and example DAGs if needed, unless SkipDirectoryCreation is true)
	if s, ok := store.(*filedag.Storage); ok {
		if err := s.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize DAG store: %w", err)
		}
	}

	return store, nil
}

// InitNamespaceScopedStores re-initializes stores to use namespace-scoped directories
// under {DataDir}/{namespaceShortID}/. Directories are created if they don't exist.
func (c *Context) InitNamespaceScopedStores(namespaceShortID string) error {
	nsDAGRunsDir := filepath.Join(c.Config.Paths.DataDir, namespaceShortID, "dag-runs")
	nsProcDir := filepath.Join(c.Config.Paths.DataDir, namespaceShortID, "proc")
	nsQueueDir := filepath.Join(c.Config.Paths.DataDir, namespaceShortID, "queue")

	for _, dir := range []string{nsDAGRunsDir, nsProcDir, nsQueueDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create namespace directory %s: %w", dir, err)
		}
	}

	c.DAGRunStore = filedagrun.New(nsDAGRunsDir,
		filedagrun.WithLatestStatusToday(c.Config.Server.LatestStatusToday),
		filedagrun.WithLocation(c.Config.Core.Location),
	)
	c.ProcStore = fileproc.New(nsProcDir)
	c.QueueStore = filequeue.New(nsQueueDir)
	c.DAGRunMgr = runtime.NewManager(c.DAGRunStore, c.ProcStore, c.Config)

	return nil
}

// ResolveNamespace reads the --namespace flag (defaulting to "default"),
// resolves it to a short ID, and re-initializes stores with namespace-scoped directories.
func (c *Context) ResolveNamespace() (string, error) {
	namespaceName, err := c.StringParam("namespace")
	if err != nil {
		return "", fmt.Errorf("failed to get namespace: %w", err)
	}
	if namespaceName == "" {
		return "", fmt.Errorf("namespace must not be empty; use --namespace flag or set a default")
	}

	namespaceShortID, err := c.NamespaceStore.Resolve(c, namespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve namespace %q: %w", namespaceName, err)
	}

	c.NamespaceShortID = namespaceShortID

	if err := c.InitNamespaceScopedStores(namespaceShortID); err != nil {
		return "", err
	}

	return namespaceName, nil
}

// ResolveNamespaceFromArg parses a "namespace/dag-name" format from the argument,
// resolves the namespace, initializes namespace-scoped stores, and returns the
// namespace name and the DAG name portion of the argument.
// If the argument contains no namespace prefix, falls back to the --namespace flag
// (default: "default").
func (c *Context) ResolveNamespaceFromArg(arg string) (namespaceName, dagName string, err error) {
	namespaceName, dagName = parseNamespaceFromArg(arg)
	if namespaceName == "" {
		namespaceName, err = c.StringParam("namespace")
		if err != nil {
			return "", "", fmt.Errorf("failed to get namespace: %w", err)
		}
		if namespaceName == "" {
			return "", "", fmt.Errorf("namespace must not be empty; use --namespace flag or 'namespace/dag' format")
		}
	}

	namespaceShortID, err := c.NamespaceStore.Resolve(c, namespaceName)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve namespace %q: %w", namespaceName, err)
	}

	c.NamespaceShortID = namespaceShortID

	if err := c.InitNamespaceScopedStores(namespaceShortID); err != nil {
		return "", "", err
	}

	return namespaceName, dagName, nil
}

// NamespacedDAGsDir returns the DAGs directory scoped to the resolved namespace.
// Falls back to the base DAGsDir if no namespace has been resolved.
func (c *Context) NamespacedDAGsDir() string {
	if c.NamespaceShortID != "" {
		return filepath.Join(c.Config.Paths.DAGsDir, c.NamespaceShortID)
	}
	return c.Config.Paths.DAGsDir
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
	baseLogDir, err := eval.String(c, c.Config.Paths.LogDir, eval.WithOSExpansion())
	if err != nil {
		return "", fmt.Errorf("failed to expand log directory: %w", err)
	}

	// Read the log directory configuration from the DAG.
	dagLogDir, err := eval.String(c, dag.LogDir, eval.WithOSExpansion())
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
	initFlags(cmd, flags...)

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

// RecordEarlyFailure records a failure in the execution history before the DAG has fully started.
// This is used for infrastructure errors like singleton conflicts or process acquisition failures.
func (c *Context) RecordEarlyFailure(dag *core.DAG, dagRunID string, err error) error {
	if dag == nil || dagRunID == "" {
		return fmt.Errorf("DAG and dag-run ID are required to record failure")
	}

	// 1. Check if a DAGRunAttempt already exists for the given run-id.
	ref := exec.NewDAGRunRef(dag.Name, dagRunID)
	attempt, findErr := c.DAGRunStore.FindAttempt(c, ref)
	if findErr != nil && !errors.Is(findErr, exec.ErrDAGRunIDNotFound) {
		return fmt.Errorf("failed to check for existing attempt: %w", findErr)
	}

	if attempt == nil {
		// 2. Create the attempt if not exists
		att, createErr := c.DAGRunStore.CreateAttempt(c, dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
		if createErr != nil {
			return fmt.Errorf("failed to create run to record failure: %w", createErr)
		}
		attempt = att
	}

	// 3. Construct the "Failed" status
	statusBuilder := transform.NewStatusBuilder(dag)
	logPath, _ := c.GenLogFileName(dag, dagRunID)
	status := statusBuilder.Create(dagRunID, core.Failed, 0, time.Now(),
		transform.WithLogFilePath(logPath),
		transform.WithFinishedAt(time.Now()),
		transform.WithError(err.Error()),
	)

	// 4. Write the status
	if err := attempt.Open(c); err != nil {
		return fmt.Errorf("failed to open attempt for recording failure: %w", err)
	}
	defer func() {
		_ = attempt.Close(c)
	}()

	if err := attempt.Write(c, status); err != nil {
		return fmt.Errorf("failed to write failed status: %w", err)
	}

	return nil
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
