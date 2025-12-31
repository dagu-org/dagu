package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// SubCmdBuilder centralizes CLI command argument construction.
type SubCmdBuilder struct {
	executable string
	configFile string
	baseEnv    config.BaseEnv
}

// NewSubCmdBuilder returns a new SubCmdBuilder initialized from cfg.
// It sets Executable to cfg.Paths.Executable, ConfigFile to cfg.Paths.ConfigFileUsed,
// and base environment to cfg.Core.BaseEnv.
func NewSubCmdBuilder(cfg *config.Config) *SubCmdBuilder {
	return &SubCmdBuilder{
		executable: cfg.Paths.Executable,
		configFile: cfg.Paths.ConfigFileUsed,
		baseEnv:    cfg.Core.BaseEnv,
	}
}

// Start creates a start command spec.
func (b *SubCmdBuilder) Start(dag *core.DAG, opts StartOptions) CmdSpec {
	args := []string{"start"}

	if opts.Params != "" {
		args = append(args, "-p", strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}

	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--run-id=%s", opts.DAGRunID))
	}
	if opts.NameOverride != "" {
		args = append(args, fmt.Sprintf("--name=%s", opts.NameOverride))
	}
	if opts.FromRunID != "" {
		args = append(args, fmt.Sprintf("--from-run-id=%s", opts.FromRunID))
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	target := dag.Location
	if opts.Target != "" {
		target = opts.Target
	}
	args = append(args, target)

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        os.Environ(),
	}
}

// Enqueue creates an enqueue command spec.
func (b *SubCmdBuilder) Enqueue(dag *core.DAG, opts EnqueueOptions) CmdSpec {
	args := []string{"enqueue"}

	if opts.Params != "" {
		args = append(args, "-p", strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--run-id=%s", opts.DAGRunID))
	}
	if opts.NameOverride != "" {
		args = append(args, fmt.Sprintf("--name=%s", opts.NameOverride))
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	if opts.Queue != "" {
		args = append(args, "--queue", opts.Queue)
	}
	args = append(args, dag.Location)

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Dequeue creates a dequeue command spec.
func (b *SubCmdBuilder) Dequeue(dag *core.DAG, dagRun execution.DAGRunRef) CmdSpec {
	queueName := dag.ProcGroup()
	args := []string{"dequeue", queueName, fmt.Sprintf("--dag-run=%s", dagRun.String())}

	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Restart creates a restart command spec.
func (b *SubCmdBuilder) Restart(dag *core.DAG, opts RestartOptions) CmdSpec {
	args := []string{"restart"}

	if opts.Quiet {
		args = append(args, "-q")
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, dag.Location)

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        b.baseEnv.AsSlice(),
	}
}

// Retry creates a retry command spec.
func (b *SubCmdBuilder) Retry(dag *core.DAG, dagRunID string, stepName string) CmdSpec {
	args := []string{"retry", fmt.Sprintf("--run-id=%s", dagRunID), "-q"}

	if stepName != "" {
		args = append(args, fmt.Sprintf("--step=%s", stepName))
	}

	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, dag.Name)

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        b.baseEnv.AsSlice(),
	}
}

// TaskStart creates a start command spec for coordinator tasks.
func (b *SubCmdBuilder) TaskStart(task *coordinatorv1.Task) CmdSpec {
	args := []string{"start", "-q"}

	// Add hierarchy flags for sub DAGs
	if task.RootDagRunId != "" {
		args = append(args, fmt.Sprintf("--root=%s:%s", task.RootDagRunName, task.RootDagRunId))
	}
	if task.ParentDagRunId != "" {
		args = append(args, fmt.Sprintf("--parent=%s:%s", task.ParentDagRunName, task.ParentDagRunId))
	}

	args = append(args, fmt.Sprintf("--run-id=%s", task.DagRunId))

	// Pass worker ID for tracking which worker executes this DAG run
	if task.WorkerId != "" {
		args = append(args, fmt.Sprintf("--worker-id=%s", task.WorkerId))
	}

	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, task.Target)

	if task.Params != "" {
		args = append(args, "--", task.Params)
	}

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        os.Environ(),
	}
}

// TaskRetry creates a retry command spec for coordinator tasks.
func (b *SubCmdBuilder) TaskRetry(task *coordinatorv1.Task) CmdSpec {
	args := []string{"retry", fmt.Sprintf("--run-id=%s", task.DagRunId), "-q"}

	if task.Step != "" {
		args = append(args, fmt.Sprintf("--step=%s", task.Step))
	}

	// Pass worker ID for tracking which worker executes this DAG run
	if task.WorkerId != "" {
		args = append(args, fmt.Sprintf("--worker-id=%s", task.WorkerId))
	}

	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	// Use RootDagRunName instead of Target, because Target may be a temporary file
	// created by the worker, but retry needs the original DAG name
	args = append(args, task.RootDagRunName)

	return CmdSpec{
		Executable: b.executable,
		Args:       args,
		Env:        os.Environ(),
	}
}

// CmdSpec describes a command to be executed with all its configuration.
type CmdSpec struct {
	Executable string
	Args       []string
	Env        []string
	Stdout     *os.File
	Stderr     *os.File
}

// StartOptions contains options for initiating a dag-run.
type StartOptions struct {
	Params   string // Parameters to pass to the DAG
	Quiet    bool   // Whether to run in quiet mode
	DAGRunID string // ID for the dag-run

	NameOverride string // Optional DAG name override
	FromRunID    string // Historic dag-run ID to use as a template
	Target       string // Optional CLI argument override (DAG name or file path)
}

// EnqueueOptions contains options for enqueuing a dag-run.
type EnqueueOptions struct {
	Params       string // Parameters to pass to the DAG
	Quiet        bool   // Whether to run in quiet mode
	DAGRunID     string // ID for the dag-run
	Queue        string // Queue name to enqueue to
	NameOverride string // Optional DAG name override
}

// RestartOptions contains options for restarting a dag-run.
type RestartOptions struct {
	Quiet bool // Whether to run in quiet mode
}

// Run executes the command and waits for it to complete.
func Run(ctx context.Context, spec CmdSpec) error {
	// nolint:gosec
	cmd := exec.CommandContext(ctx, spec.Executable, spec.Args...)
	cmdutil.SetupCommand(cmd)
	cmd.Env = spec.Env

	if spec.Stdout != nil {
		cmd.Stdout = spec.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	if spec.Stderr != nil {
		cmd.Stderr = spec.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// Start executes the command without waiting for it to complete.
func Start(ctx context.Context, spec CmdSpec) error {
	// nolint:gosec
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmdutil.SetupCommand(cmd)
	cmd.Env = spec.Env

	if spec.Stdout != nil {
		cmd.Stdout = spec.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if spec.Stderr != nil {
		cmd.Stderr = spec.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	go execWithRecovery(ctx, func() {
		_ = cmd.Wait()
	})

	return nil
}
