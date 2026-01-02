package execution

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/stringutil"
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
	Envs               map[string]string
	SecretEnvs         map[string]string // Secret environment variables (highest priority)
	CoordinatorCli     Dispatcher
	Shell              string // Default shell for this DAG (from DAG.Shell)
	LogEncodingCharset string // Character encoding for log files (e.g., "utf-8", "shift_jis", "euc-jp")
}

// UserEnvsMap returns only user-defined environment variables as a map,
// excluding OS environment (BaseEnv). Use this for isolated execution environments.
// Precedence: SecretEnvs > Envs > DAG.Env
func (e Context) UserEnvsMap() map[string]string {
	result := make(map[string]string)

	// Parse DAG.Env (lowest priority)
	for _, env := range e.DAG.Env {
		key, value, found := strings.Cut(env, "=")
		if found {
			result[key] = value
		}
	}

	// Add computed envs
	for k, v := range e.Envs {
		result[k] = v
	}

	// Secrets last (highest priority)
	for k, v := range e.SecretEnvs {
		result[k] = v
	}

	return result
}

// DAGRunRef returns the DAGRunRef for the current DAG context.
func (e Context) DAGRunRef() DAGRunRef {
	return NewDAGRunRef(e.DAG.Name, e.DAGRunID)
}

// AllEnvs returns every environment variable as "key=value" with precedence:
// BaseEnv < DAG.Env < e.Envs < SecretEnvs < runtime metadata (e.g., DAGU_PARAMS_JSON).
func (e Context) AllEnvs() []string {
	distinctEntries := make(map[string]string)

	for k, v := range stringutil.KeyValuesToMap(e.BaseEnv.AsSlice()) {
		distinctEntries[k] = v
	}
	for k, v := range stringutil.KeyValuesToMap(e.DAG.Env) {
		distinctEntries[k] = v
	}
	for k, v := range e.Envs {
		distinctEntries[k] = v
	}
	for k, v := range e.SecretEnvs {
		distinctEntries[k] = v
	}

	// Add DAGU_PARAMS_JSON with JSON encoded params when available
	if e.DAG.ParamsJSON != "" {
		distinctEntries[EnvKeyDAGParamsJSON] = e.DAG.ParamsJSON
	}

	var envs []string
	for k, v := range distinctEntries {
		envs = append(envs, k+"="+v)
	}

	return envs
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
}

// contextOptions holds optional configuration for NewContext.
type contextOptions struct {
	db                 Database
	rootDAGRun         DAGRunRef
	params             []string
	coordinator        Dispatcher
	secretEnvs         []string
	logEncodingCharset string
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
	for k, v := range stringutil.KeyValuesToMap(options.params) {
		envs[k] = v
	}
	for k, v := range stringutil.KeyValuesToMap(dag.Env) {
		envs[k] = v
	}

	return context.WithValue(ctx, dagCtxKey{}, Context{
		RootDAGRun:         options.rootDAGRun,
		DAG:                dag,
		DB:                 options.db,
		Envs:               envs,
		SecretEnvs:         stringutil.KeyValuesToMap(options.secretEnvs),
		DAGRunID:           dagRunID,
		BaseEnv:            config.GetBaseEnv(ctx),
		CoordinatorCli:     options.coordinator,
		Shell:              dag.Shell,
		LogEncodingCharset: options.logEncodingCharset,
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
