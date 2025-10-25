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

// NewSubCmdBuilder creates a new CmdBuilder instance.
func NewSubCmdBuilder(cfg *config.Config) *SubCmdBuilder {
	return &SubCmdBuilder{
		executable: cfg.Paths.Executable,
		configFile: cfg.Global.ConfigFileUsed,
		baseEnv:    cfg.Global.BaseEnv,
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
	if opts.NoQueue {
		args = append(args, "--no-queue")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--run-id=%s", opts.DAGRunID))
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, dag.Location)

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
func (b *SubCmdBuilder) Dequeue(_ *core.DAG, dagRun execution.DAGRunRef) CmdSpec {
	args := []string{"dequeue", fmt.Sprintf("--dag-run=%s", dagRun.String())}

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
func (b *SubCmdBuilder) Retry(dag *core.DAG, dagRunID string, stepName string, disableMaxActiveRuns bool) CmdSpec {
	args := []string{"retry", fmt.Sprintf("--run-id=%s", dagRunID)}

	if stepName != "" {
		args = append(args, fmt.Sprintf("--step=%s", stepName))
	}
	if disableMaxActiveRuns {
		args = append(args, "--disable-max-active-runs")
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
	args := []string{"start"}

	// Add hierarchy flags for child DAGs
	if task.RootDagRunId != "" {
		args = append(args, fmt.Sprintf("--root=%s:%s", task.RootDagRunName, task.RootDagRunId))
	}
	if task.ParentDagRunId != "" {
		args = append(args, fmt.Sprintf("--parent=%s:%s", task.ParentDagRunName, task.ParentDagRunId))
	}

	args = append(args, fmt.Sprintf("--run-id=%s", task.DagRunId), "--no-queue")

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
	args := []string{"retry", fmt.Sprintf("--run-id=%s", task.DagRunId), "--no-queue"}

	if task.Step != "" {
		args = append(args, fmt.Sprintf("--step=%s", task.Step))
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
	NoQueue  bool   // Do not allow queueing
}

// EnqueueOptions contains options for enqueuing a dag-run.
type EnqueueOptions struct {
	Params   string // Parameters to pass to the DAG
	Quiet    bool   // Whether to run in quiet mode
	DAGRunID string // ID for the dag-run
	Queue    string // Queue name to enqueue to
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

	// If custom streams are provided, use them and call Run()
	if spec.Stdout != nil || spec.Stderr != nil {
		if spec.Stdout != nil {
			cmd.Stdout = spec.Stdout
		}
		if spec.Stderr != nil {
			cmd.Stderr = spec.Stderr
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
		return nil
	}

	// Otherwise capture output with CombinedOutput()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\noutput: %s", err, output)
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
