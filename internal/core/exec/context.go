package exec

import (
	"context"
	"encoding/json"
	"io"
	"maps"

	"github.com/dagu-org/dagu/internal/agent/iface"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// Context contains the execution metadata for a dag-run.
type Context struct {
	DAGRunID           string
	RootDAGRun         DAGRunRef
	DAG                *core.DAG
	DB                 Database
	BaseEnv            *config.BaseEnv
	EnvScope           *eval.EnvScope // Unified environment scope - THE single source for all env vars
	CoordinatorCli     Dispatcher
	Shell              string               // Default shell for this DAG (from DAG.Shell)
	LogEncodingCharset string               // Character encoding for log files (e.g., "utf-8", "shift_jis", "euc-jp")
	LogWriterFactory   LogWriterFactory     // For remote log streaming (nil = use local files)
	DefaultExecMode    config.ExecutionMode // Server-level default execution mode (local or distributed)
	AgentConfigStore   iface.ConfigStore
	AgentModelStore    iface.ModelStore
	AgentMemoryStore   iface.MemoryStore
}

// LogWriterFactory creates log writers for step stdout/stderr.
// It abstracts where logs are written, allowing for:
// - Local file-based storage (default)
// - Remote streaming to coordinator (shared-nothing mode)
type LogWriterFactory interface {
	// NewStepWriter creates a writer for a step's log output.
	// stepName identifies the step, streamType should be StreamTypeStdout or StreamTypeStderr.
	NewStepWriter(ctx context.Context, stepName string, streamType int) io.WriteCloser
}

// Stream type constants for LogWriterFactory.NewStepWriter
const (
	// StreamTypeStdout indicates stdout stream
	StreamTypeStdout = 1
	// StreamTypeStderr indicates stderr stream
	StreamTypeStderr = 2
)

// UserEnvsMap returns only user-defined environment variables as a map,
// excluding OS environment (BaseEnv). Use this for isolated execution environments.
func (e Context) UserEnvsMap() map[string]string {
	if e.EnvScope == nil {
		return make(map[string]string)
	}
	return e.EnvScope.AllUserEnvs()
}

// DAGRunRef returns the DAGRunRef for the current DAG context.
func (e Context) DAGRunRef() DAGRunRef {
	return NewDAGRunRef(e.DAG.Name, e.DAGRunID)
}

// AllEnvs returns every environment variable as "key=value" strings.
// Uses EnvScope as the single source of truth for all env vars.
func (e Context) AllEnvs() []string {
	if e.EnvScope == nil {
		return nil
	}
	return e.EnvScope.ToSlice()
}

// Database is the interface for accessing the database to retrieve DAGs and dag-run statuses.
// This interface abstracts the underlying storage mechanism, allowing for different implementations (e.g., SQL, NoSQL, in-memory).
type Database interface {
	// GetDAG retrieves a DAG by its name.
	GetDAG(ctx context.Context, name string) (*core.DAG, error)
	// GetSubDAGRunStatus retrieves the status of a sub dag-run by its ID and the root dag-run reference.
	GetSubDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (*RunStatus, error)
	// IsSubDAGRunCompleted checks if a sub dag-run has completed.
	IsSubDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (bool, error)
	// RequestChildCancel requests cancellation of a sub dag-run.
	RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) error
}

// SubDAGRunStatus is an interface that represents the status of a sub dag-run.
type RunStatus struct {
	// Name represents the name of the executed DAG.
	Name string
	// DAGRunID is the ID of the dag-run.
	DAGRunID string
	// Params is the parameters of the DAG.
	Params string
	// Outputs is the outputs of the dag-run.
	Outputs map[string]string
	// Status is the execution status of the dag-run.
	Status core.Status
}

// MarshalJSON implements the json.Marshaler interface for RunStatus.
func (r *RunStatus) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(struct {
		Name     string            `json:"name,omitempty"`
		DAGRunID string            `json:"dagRunId,omitempty"`
		Params   string            `json:"params,omitempty"`
		Outputs  map[string]string `json:"outputs,omitzero"`
		Status   string            `json:"status"`
	}{
		Name:     r.Name,
		DAGRunID: r.DAGRunID,
		Params:   r.Params,
		Outputs:  r.Outputs,
		Status:   r.Status.String(),
	}, "", "  ")
}

// Dispatcher defines the interface for coordinator operations
type Dispatcher interface {
	// Dispatch sends a task to the coordinator
	Dispatch(ctx context.Context, task *coordinatorv1.Task) error

	// Cleanup cleans up any resources used by the coordinator client
	Cleanup(ctx context.Context) error

	// GetDAGRunStatus retrieves the status of a DAG run from the coordinator.
	// Used by parent DAGs to poll status of remote sub-DAGs.
	// For sub-DAG queries, provide rootRef to look up the status under the root DAG run.
	// Returns (nil, nil) if the DAG run is not found.
	GetDAGRunStatus(ctx context.Context, dagName, dagRunID string, rootRef *DAGRunRef) (*coordinatorv1.GetDAGRunStatusResponse, error)

	// RequestCancel requests cancellation of a DAG run through the coordinator.
	// Used in shared-nothing mode for sub-DAG cancellation where the parent
	// worker cannot directly access the sub-DAG's attempt.
	RequestCancel(ctx context.Context, dagName, dagRunID string, rootRef *DAGRunRef) error
}

// contextOptions holds optional configuration for NewContext.
type contextOptions struct {
	db                 Database
	rootDAGRun         DAGRunRef
	params             []string
	coordinator        Dispatcher
	secretEnvs         []string
	logEncodingCharset string
	logWriterFactory   LogWriterFactory
	defaultExecMode    config.ExecutionMode
	agentConfigStore   iface.ConfigStore
	agentModelStore    iface.ModelStore
	agentMemoryStore   iface.MemoryStore
}

// ContextOption configures optional parameters for NewContext.
type ContextOption func(*contextOptions)

// WithDatabase sets the database interface.
func WithDatabase(db Database) ContextOption {
	return func(o *contextOptions) {
		o.db = db
	}
}

// WithRootDAGRun sets the root DAG run reference for sub-DAG execution.
func WithRootDAGRun(ref DAGRunRef) ContextOption {
	return func(o *contextOptions) {
		o.rootDAGRun = ref
	}
}

// WithParams sets runtime parameters.
func WithParams(params []string) ContextOption {
	return func(o *contextOptions) {
		o.params = params
	}
}

// WithCoordinator sets the coordinator dispatcher for distributed execution.
func WithCoordinator(cli Dispatcher) ContextOption {
	return func(o *contextOptions) {
		o.coordinator = cli
	}
}

// WithSecrets sets secret environment variables.
func WithSecrets(secrets []string) ContextOption {
	return func(o *contextOptions) {
		o.secretEnvs = secrets
	}
}

// WithLogEncoding sets the log file character encoding.
func WithLogEncoding(charset string) ContextOption {
	return func(o *contextOptions) {
		o.logEncodingCharset = charset
	}
}

// WithLogWriterFactory sets the log writer factory for remote log streaming.
// When set, logs are streamed to the coordinator instead of written to local files.
func WithLogWriterFactory(factory LogWriterFactory) ContextOption {
	return func(o *contextOptions) {
		o.logWriterFactory = factory
	}
}

// WithDefaultExecMode sets the server-level default execution mode.
func WithDefaultExecMode(mode config.ExecutionMode) ContextOption {
	return func(o *contextOptions) {
		o.defaultExecMode = mode
	}
}

// WithAgentConfigStore sets the agent configuration store.
func WithAgentConfigStore(store iface.ConfigStore) ContextOption {
	return func(o *contextOptions) {
		o.agentConfigStore = store
	}
}

// WithAgentModelStore sets the agent model store.
func WithAgentModelStore(store iface.ModelStore) ContextOption {
	return func(o *contextOptions) {
		o.agentModelStore = store
	}
}

// WithAgentMemoryStore sets the agent memory store.
func WithAgentMemoryStore(store iface.MemoryStore) ContextOption {
	return func(o *contextOptions) {
		o.agentMemoryStore = store
	}
}

// NewContext creates a new context with DAG execution metadata.
// Required: ctx, dag, dagRunID, logFile
// Optional: use ContextOption functions (WithDatabase, WithParams, etc.)
func NewContext(
	ctx context.Context,
	dag *core.DAG,
	dagRunID string,
	logFile string,
	opts ...ContextOption,
) context.Context {
	// Apply options
	options := &contextOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Build environment variables
	envs := map[string]string{
		EnvKeyDAGRunLogFile: logFile,
		EnvKeyDAGRunID:      dagRunID,
		EnvKeyDAGName:       dag.Name,
	}
	maps.Copy(envs, stringutil.KeyValuesToMap(options.params))
	maps.Copy(envs, stringutil.KeyValuesToMap(dag.Env))

	secretEnvs := stringutil.KeyValuesToMap(options.secretEnvs)

	// Build EnvScope with proper source tracking and layering
	// Precedence (highest to lowest): Secrets > DAG Env > Params > OS
	scope := eval.NewEnvScope(nil, true) // OS layer
	scope = scope.WithEntries(envs, eval.EnvSourceDAGEnv)
	if len(secretEnvs) > 0 {
		scope = scope.WithEntries(secretEnvs, eval.EnvSourceSecret)
	}

	return context.WithValue(ctx, dagCtxKey{}, Context{
		RootDAGRun:         options.rootDAGRun,
		DAG:                dag,
		DB:                 options.db,
		EnvScope:           scope,
		DAGRunID:           dagRunID,
		BaseEnv:            config.GetBaseEnv(ctx),
		CoordinatorCli:     options.coordinator,
		Shell:              dag.Shell,
		LogEncodingCharset: options.logEncodingCharset,
		LogWriterFactory:   options.logWriterFactory,
		DefaultExecMode:    options.defaultExecMode,
		AgentConfigStore:   options.agentConfigStore,
		AgentModelStore:    options.agentModelStore,
		AgentMemoryStore:   options.agentMemoryStore,
	})
}

// WithContext returns a new context with the given DAGContext.
// This is useful for tests that need to set up a DAGContext directly.
func WithContext(ctx context.Context, rCtx Context) context.Context {
	return context.WithValue(ctx, dagCtxKey{}, rCtx)
}

// GetContext retrieves the DAGContext from the context.
func GetContext(ctx context.Context) Context {
	value := ctx.Value(dagCtxKey{})
	if value == nil {
		logger.Error(ctx, "DAGContext not found in context")
		return Context{}
	}
	execEnv, ok := value.(Context)
	if !ok {
		logger.Error(ctx, "Invalid DAGContext type in context")
		return Context{}
	}
	return execEnv
}

type dagCtxKey struct{}
