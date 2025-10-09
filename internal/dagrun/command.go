package dagrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// CmdSpec describes a command to be executed with all its configuration.
type CmdSpec struct {
	Args       []string
	WorkingDir string
	Env        []string
	Stdout     *os.File
	Stderr     *os.File
}

// CmdRunner executes commands synchronously or asynchronously.
type CmdRunner interface {
	// Run executes the command and waits for it to complete.
	Run(ctx context.Context, spec CmdSpec) error
	// Start executes the command without waiting for it to complete.
	Start(ctx context.Context, spec CmdSpec) error
}

// defaultCommandRunner implements CommandRunner using os/exec.
type defaultCommandRunner struct {
	executable string
}

func newCommandRunner(executable string) CmdRunner {
	return &defaultCommandRunner{executable: executable}
}

func (r *defaultCommandRunner) Run(ctx context.Context, spec CmdSpec) error {
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

func (r *defaultCommandRunner) Start(ctx context.Context, spec CmdSpec) error {
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

func NewCmdBuilder(configFile string) *CmdBuilder {
	return &CmdBuilder{configFile: configFile}
}

// Start creates a start command spec.
func (b *CmdBuilder) Start(dag *digraph.DAG, opts StartOptions) CmdSpec {
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
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        os.Environ(),
	}
}

// Enqueue creates an enqueue command spec.
func (b *CmdBuilder) Enqueue(dag *digraph.DAG, opts EnqueueOptions) CmdSpec {
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
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Dequeue creates a dequeue command spec.
func (b *CmdBuilder) Dequeue(dag *digraph.DAG, dagRun digraph.DAGRunRef) CmdSpec {
	args := []string{"dequeue", fmt.Sprintf("--dag-run=%s", dagRun.String())}

	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}

	return CmdSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        os.Environ(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Restart creates a restart command spec.
func (b *CmdBuilder) Restart(dag *digraph.DAG, opts RestartOptions) CmdSpec {
	args := []string{"restart"}

	if opts.Quiet {
		args = append(args, "-q")
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, dag.Location)

	return CmdSpec{
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        os.Environ(),
	}
}

// Retry creates a retry command spec.
func (b *CmdBuilder) Retry(dag *digraph.DAG, dagRunID string, stepName string, disableMaxActiveRuns bool) CmdSpec {
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
		Args:       args,
		WorkingDir: dag.WorkingDir,
		Env:        os.Environ(),
	}
}

// TaskStart creates a start command spec for coordinator tasks.
func (b *CmdBuilder) TaskStart(task *coordinatorv1.Task) CmdSpec {
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
		Args: args,
		Env:  os.Environ(),
	}
}

// TaskRetry creates a retry command spec for coordinator tasks.
func (b *CmdBuilder) TaskRetry(task *coordinatorv1.Task) CmdSpec {
	args := []string{"retry", fmt.Sprintf("--run-id=%s", task.DagRunId)}

	if task.Step != "" {
		args = append(args, fmt.Sprintf("--step=%s", task.Step))
	}
	if b.configFile != "" {
		args = append(args, "--config", b.configFile)
	}
	args = append(args, task.Target)

	return CmdSpec{
		Args: args,
		Env:  os.Environ(),
	}
}
