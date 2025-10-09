package dagrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// CommandSpec describes a command to be executed with all its configuration.
type CommandSpec struct {
	Args       []string
	WorkingDir string
	Env        []string
	Stdout     *os.File
	Stderr     *os.File
}

// CommandRunner executes commands synchronously or asynchronously.
type CommandRunner interface {
	// Run executes the command and waits for it to complete.
	Run(ctx context.Context, spec CommandSpec) error
	// Start executes the command without waiting for it to complete.
	Start(ctx context.Context, spec CommandSpec) error
}

// defaultCommandRunner implements CommandRunner using os/exec.
type defaultCommandRunner struct {
	executable string
}

func newCommandRunner(executable string) CommandRunner {
	return &defaultCommandRunner{executable: executable}
}

func (r *defaultCommandRunner) Run(ctx context.Context, spec CommandSpec) error {
	// nolint:gosec
	cmd := exec.CommandContext(ctx, r.executable, spec.Args...)
	executor.SetupCommand(cmd)
	cmd.Dir = spec.WorkingDir
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

func (r *defaultCommandRunner) Start(ctx context.Context, spec CommandSpec) error {
	// nolint:gosec
	cmd := exec.Command(r.executable, spec.Args...)
	executor.SetupCommand(cmd)
	cmd.Dir = spec.WorkingDir
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

// CmdBuilder centralizes CLI command argument construction.
type CmdBuilder struct {
	configFile string
}

func NewCmdBuilder(ctx context.Context) *CmdBuilder {
	return &CmdBuilder{configFile: config.ConfigFileUsed(ctx)}
}

// Start creates a start command spec.
func (b *CmdBuilder) Start(ctx context.Context, dag *digraph.DAG, opts StartOptions) CommandSpec {
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

	return CommandSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        config.GetBaseEnv(ctx).AsSlice(),
	}
}

// Enqueue creates an enqueue command spec.
func (b *CmdBuilder) Enqueue(ctx context.Context, dag *digraph.DAG, opts EnqueueOptions) CommandSpec {
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

	return CommandSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        config.GetBaseEnv(ctx).AsSlice(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Dequeue creates a dequeue command spec.
func (b *CmdBuilder) Dequeue(ctx context.Context, dag *digraph.DAG, dagRun digraph.DAGRunRef) CommandSpec {
	args := []string{"dequeue", fmt.Sprintf("--dag-run=%s", dagRun.String())}

	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}

	return CommandSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        config.GetBaseEnv(ctx).AsSlice(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Restart creates a restart command spec.
func (b *CmdBuilder) Restart(ctx context.Context, dag *digraph.DAG, opts RestartOptions) CommandSpec {
	args := []string{"restart"}

	if opts.Quiet {
		args = append(args, "-q")
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, dag.Location)

	return CommandSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        config.GetBaseEnv(ctx).AsSlice(),
	}
}

// Retry creates a retry command spec.
func (b *CmdBuilder) Retry(ctx context.Context, dag *digraph.DAG, dagRunID string, stepName string, disableMaxActiveRuns bool) CommandSpec {
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

	return CommandSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        config.GetBaseEnv(ctx).AsSlice(),
	}
}

// TaskStart creates a start command spec for coordinator tasks.
func (b *CmdBuilder) TaskStart(ctx context.Context, task *coordinatorv1.Task) CommandSpec {
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

	return CommandSpec{
		Args: args,
		Env:  config.GetBaseEnv(ctx).AsSlice(),
	}
}

// TaskRetry creates a retry command spec for coordinator tasks.
func (b *CmdBuilder) TaskRetry(ctx context.Context, task *coordinatorv1.Task) CommandSpec {
	args := []string{"retry", fmt.Sprintf("--run-id=%s", task.DagRunId)}

	if task.Step != "" {
		args = append(args, fmt.Sprintf("--step=%s", task.Step))
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, task.Target)

	return CommandSpec{
		Args: args,
		Env:  config.GetBaseEnv(ctx).AsSlice(),
	}
}
