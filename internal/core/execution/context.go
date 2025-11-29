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

// DAGContext contains the execution metadata for a dag-run.
type DAGContext struct {
	DAGRunID       string
	RootDAGRun     DAGRunRef
	DAG            *core.DAG
	DB             Database
	BaseEnv        *config.BaseEnv
	Envs           map[string]string
	SecretEnvs     map[string]string // Secret environment variables (highest priority)
	CoordinatorCli Dispatcher
	Shell          string // Default shell for this DAG (from DAG.Shell)
}

// UserEnvsMap returns only user-defined environment variables as a map,
// excluding OS environment (BaseEnv). Use this for isolated execution environments.
// Precedence: SecretEnvs > Envs > DAG.Env
func (e DAGContext) UserEnvsMap() map[string]string {
	result := make(map[string]string)

	// Parse DAG.Env (lowest priority)
	for _, env := range e.DAG.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
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

// AllEnvs returns all environment variables as a slice of strings in "key=value" format.
// Includes OS environment (BaseEnv). Use this for command executor and DAG runner.
// Secrets have the highest priority and are appended last.
func (e DAGContext) AllEnvs() []string {
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

// SetupDAGContext initializes and returns a new context with DAG execution metadata.
func SetupDAGContext(ctx context.Context, dag *core.DAG, db Database, rootDAGRun DAGRunRef, dagRunID, logFile string, params []string, coordinatorCli Dispatcher, secretEnvs []string) context.Context {
	var envs = map[string]string{
		EnvKeyDAGRunLogFile: logFile,
		EnvKeyDAGRunID:      dagRunID,
		EnvKeyDAGName:       dag.Name,
	}

	for k, v := range stringutil.KeyValuesToMap(params) {
		envs[k] = v
	}

	for k, v := range stringutil.KeyValuesToMap(dag.Env) {
		envs[k] = v
	}

	secretEnvsMap := stringutil.KeyValuesToMap(secretEnvs)

	return context.WithValue(ctx, dagCtxKey{}, DAGContext{
		RootDAGRun:     rootDAGRun,
		DAG:            dag,
		DB:             db,
		Envs:           envs,
		SecretEnvs:     secretEnvsMap,
		DAGRunID:       dagRunID,
		BaseEnv:        config.GetBaseEnv(ctx),
		CoordinatorCli: coordinatorCli,
		Shell:          dag.Shell,
	})
}

// GetDAGContext retrieves the DAGContext from the context.
func GetDAGContext(ctx context.Context) DAGContext {
	value := ctx.Value(dagCtxKey{})
	if value == nil {
		logger.Error(ctx, "DAGContext not found in context")
		return DAGContext{}
	}
	execEnv, ok := value.(DAGContext)
	if !ok {
		logger.Error(ctx, "Invalid DAGContext type in context")
		return DAGContext{}
	}
	return execEnv
}

type dagCtxKey struct{}
