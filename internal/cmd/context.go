package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/localdag"
	"github.com/dagu-org/dagu/internal/persistence/localhistory"
	"github.com/dagu-org/dagu/internal/persistence/localproc"
	"github.com/dagu-org/dagu/internal/persistence/localqueue/prototype"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Context holds the configuration for a command.
type Context struct {
	context.Context

	Command      *cobra.Command
	Flags        []commandLineFlag
	Config       *config.Config
	Quiet        bool
	HistoryStore models.HistoryStore
	HistoryMgr   history.Manager
	ProcStore    models.ProcStore
	QueueStore   models.QueueStore
}

// LogToFile creates a new logger context with a file writer.
func (c *Context) LogToFile(f *os.File) {
	var opts []logger.Option
	if c.Config.Global.Debug {
		opts = append(opts, logger.WithDebug())
	}
	if c.Quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if c.Config.Global.LogFormat != "" {
		opts = append(opts, logger.WithFormat(c.Config.Global.LogFormat))
	}
	if f != nil {
		opts = append(opts, logger.WithWriter(f))
	}
	c.Context = logger.WithLogger(c.Context, logger.NewLogger(opts...))
}

// NewContext initializes the application setup by loading configuration,
// setting up logger context, and logging any warnings.
func NewContext(cmd *cobra.Command, flags []commandLineFlag) (*Context, error) {
	ctx := cmd.Context()

	bindFlags(cmd, flags...)

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return nil, fmt.Errorf("failed to get quiet flag: %w", err)
	}

	var configLoaderOpts []config.ConfigLoaderOption

	// Use a custom config file if provided via the viper flag "config"
	if cfgPath := viper.GetString("config"); cfgPath != "" {
		configLoaderOpts = append(configLoaderOpts, config.WithConfigFile(cfgPath))
	}

	cfg, err := config.Load(configLoaderOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create a logger context based on config and quiet mode
	var opts []logger.Option
	if cfg.Global.Debug || os.Getenv("DEBUG") != "" {
		opts = append(opts, logger.WithDebug())
	}
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if cfg.Global.LogFormat != "" {
		opts = append(opts, logger.WithFormat(cfg.Global.LogFormat))
	}
	ctx = logger.WithLogger(ctx, logger.NewLogger(opts...))

	// Log any warnings collected during configuration loading
	for _, w := range cfg.Warnings {
		logger.Warn(ctx, w)
	}

	// Initialize history repository and history manager
	hrOpts := []localhistory.HistoryStoreOption{
		localhistory.WithLatestStatusToday(cfg.Server.LatestStatusToday),
	}

	switch cmd.Name() {
	case "server", "scheduler", "start-all":
		// For long-running process, we setup file cache for better performance
		hc := fileutil.NewCache[*models.Status](0, time.Hour*12)
		hc.StartEviction(ctx)
		hrOpts = append(hrOpts, localhistory.WithHistoryFileCache(hc))
	}

	hs := localhistory.New(cfg.Paths.HistoryDir, hrOpts...)
	hm := history.New(hs, cfg.Paths.Executable, cfg.Global.WorkDir)
	ps := localproc.New(cfg.Paths.ProcDir)
	qs := prototype.New(cfg.Paths.QueueDir)

	return &Context{
		Context:      ctx,
		Command:      cmd,
		Config:       cfg,
		Quiet:        quiet,
		HistoryStore: hs,
		HistoryMgr:   hm,
		Flags:        flags,
		ProcStore:    ps,
		QueueStore:   qs,
	}, nil
}

// HistoryManager initializes a HistoryManager using the provided options. If not supplied,
func (c *Context) HistoryManager(hr models.HistoryStore) history.Manager {
	return history.New(
		hr,
		c.Config.Paths.Executable,
		c.Config.Global.WorkDir,
	)
}

// NewServer creates and returns a new web UI NewServer.
// It initializes in-memory caches for DAGs and runstore, and uses them in the client.
func (c *Context) NewServer() (*frontend.Server, error) {
	dc := fileutil.NewCache[*digraph.DAG](0, time.Hour*12)
	dc.StartEviction(c)

	dr, err := c.dagStore(dc, nil)
	if err != nil {
		return nil, err
	}

	return frontend.NewServer(c.Config, dr, c.HistoryStore, c.HistoryMgr), nil
}

// NewScheduler creates a new NewScheduler instance using the default client.
// It builds a DAG job manager to handle scheduled executions.
func (c *Context) NewScheduler() (*scheduler.Scheduler, error) {
	cache := fileutil.NewCache[*digraph.DAG](0, time.Hour*12)
	cache.StartEviction(c)

	dr, err := c.dagStore(cache, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DAG client: %w", err)
	}

	m := scheduler.NewEntryReader(c.Config.Paths.DAGsDir, dr, c.HistoryMgr, c.Config.Paths.Executable, c.Config.Global.WorkDir)
	return scheduler.New(c.Config, m, c.HistoryMgr, c.HistoryStore, c.QueueStore, c.ProcStore), nil
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
func (c *Context) dagStore(cache *fileutil.Cache[*digraph.DAG], searchPaths []string) (models.DAGStore, error) {
	dir := c.Config.Paths.DAGsDir
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create DAGs directory %s: %w", dir, err)
		}
	}

	// Create a flag store based on the suspend flags directory.
	return localdag.New(
		c.Config.Paths.DAGsDir,
		localdag.WithFlagsBaseDir(c.Config.Paths.SuspendFlagsDir),
		localdag.WithSearchPaths(searchPaths),
		localdag.WithFileCache(cache),
	), nil
}

// OpenLogFile creates and opens a log file for a given workflow.
// It evaluates the log directory, validates settings, creates the log directory,
// builds a filename using the current timestamp and workflow ID, and then opens the file.
func (c *Context) OpenLogFile(
	dag *digraph.DAG,
	workflowID string,
) (*os.File, error) {
	logPath, err := c.GenLogFileName(dag, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate log file name: %w", err)
	}
	return fileutil.OpenOrCreateFile(logPath)
}

// GenLogFileName generates a log file name based on the DAG and workflow ID.
func (c *Context) GenLogFileName(dag *digraph.DAG, workflowID string) (string, error) {
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
		BaseDir:    baseLogDir,
		DAGLogDir:  dagLogDir,
		Name:       dag.Name,
		WorkflowID: workflowID,
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

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Setup cpu profiling if enabled.
		if cpuProfileEnabled, _ := cmd.Flags().GetBool("cpu-profile"); cpuProfileEnabled {
			f, err := os.Create("cpu.prof")
			if err != nil {
				fmt.Printf("Failed to create CPU profile file: %v\n", err)
				os.Exit(1)
			}
			pprof.StartCPUProfile(f)
			defer func() {
				pprof.StopCPUProfile()
				if err := f.Close(); err != nil {
					fmt.Printf("Failed to close CPU profile file: %v\n", err)
				}
			}()
		}

		ctx, err := NewContext(cmd, flags)
		if err != nil {
			fmt.Printf("Initialization error: %v\n", err)
			os.Exit(1)
		}
		if err := runFunc(ctx, args); err != nil {
			logger.Error(ctx.Context, "Command failed", "err", err)
			os.Exit(1)
		}
		return nil
	}

	return cmd
}

// genWorkflowID creates a new UUID string to be used as a workflow IDentifier.
func genWorkflowID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// validateWorkflowID checks if the workflow ID is valid and not empty.
func validateWorkflowID(workflowID string) error {
	if workflowID == "" {
		return ErrWorkflowIDRequired
	}
	if !regexWorkflowID.MatchString(workflowID) {
		return ErrWorkflowIDFormat
	}
	if len(workflowID) > maxWorkflowIDLen {
		return ErrWorkflowIDTooLong
	}
	return nil
}

// regexWorkflowID is a regular expression to validate workflow IDs.
// It allows alphanumeric characters, hyphens, and underscores.
var regexWorkflowID = regexp.MustCompile(`^[-a-zA-Z0-9_]+$`)

// maxWorkflowIDLen is the max length of the workflow ID
const maxWorkflowIDLen = 60

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
	BaseDir    string // Base directory for logs.
	DAGLogDir  string // Optional alternative log directory specified by the DAG definition.
	Name       string // Name of the workflow; used for generating a safe directory name.
	WorkflowID string // Unique workflow ID used in the filename.
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
	logDir := filepath.Join(baseDir, safeName, "workflow_"+utcTimestamp+"_"+cfg.WorkflowID)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return "", fmt.Errorf("failed to initialize directory %s: %w", logDir, err)
	}

	return logDir, nil
}

// LogFile constructs the log filename using the prefix, safe DAG name, current timestamp,
// and a truncated version of the workflow ID.
func (cfg LogConfig) LogFile() string {
	timestamp := time.Now().Format("20060102.15:04:05.000")
	truncatedWorkflowID := stringutil.TruncString(cfg.WorkflowID, 8)

	return fmt.Sprintf("workflow_%s.%s.log",
		timestamp,
		truncatedWorkflowID,
	)
}
