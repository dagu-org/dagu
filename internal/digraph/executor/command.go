package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

var _ Executor = (*commandExecutor)(nil)
var _ ExitCoder = (*commandExecutor)(nil)

type commandExecutor struct {
	mu         sync.Mutex
	config     *commandConfig
	cmd        *exec.Cmd
	scriptFile string
	exitCode   int
}

// ExitCode implements ExitCoder.
func (e *commandExecutor) ExitCode() int {
	return e.exitCode
}

func (e *commandExecutor) Run(ctx context.Context) error {
	e.mu.Lock()

	if len(e.config.Dir) > 0 && !fileutil.FileExists(e.config.Dir) {
		e.mu.Unlock()
		return fmt.Errorf("directory does not exist: %s", e.config.Dir)
	}

	if e.config.Script != "" {
		scriptFile, err := setupScript(ctx, digraph.Step{Dir: e.config.Dir, Script: e.config.Script})
		if err != nil {
			e.mu.Unlock()
			return fmt.Errorf("failed to setup script: %w", err)
		}
		e.scriptFile = scriptFile
		defer func() {
			// Remove the temporary script file after the command has finished
			_ = os.Remove(scriptFile)
		}()
	}
	cmd, err := e.config.newCmd(ctx, e.scriptFile)
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("failed to create command: %w", err)
	}

	e.cmd = cmd

	if err := e.cmd.Start(); err != nil {
		e.exitCode = exitCodeFromError(err)
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()

	if err := e.cmd.Wait(); err != nil {
		e.exitCode = exitCodeFromError(err)
		return err
	}

	return nil
}

func (e *commandExecutor) SetStdout(out io.Writer) {
	e.config.Stdout = out
}

func (e *commandExecutor) SetStderr(out io.Writer) {
	e.config.Stderr = out
}

func (e *commandExecutor) Kill(sig os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		return syscall.Kill(-e.cmd.Process.Pid, sig.(syscall.Signal))
	}

	return nil
}

type commandConfig struct {
	Ctx              context.Context
	Dir              string
	Command          string
	Args             []string
	Script           string
	ShellCommand     string
	ShellCommandArgs string
	Stdout           io.Writer
	Stderr           io.Writer
}

func (cfg *commandConfig) newCmd(ctx context.Context, scriptFile string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch {
	case cfg.Command != "" && scriptFile != "":
		cmd = exec.CommandContext(cfg.Ctx, cfg.Command, append(cfg.Args, scriptFile)...) // nolint: gosec

	case cfg.ShellCommand != "" && scriptFile != "":
		// If script is provided ignore the shell command args

		cmd = exec.CommandContext(cfg.Ctx, cfg.ShellCommand, scriptFile) // nolint: gosec

	case cfg.ShellCommand != "" && cfg.ShellCommandArgs != "":
		// nolint: gosec
		cmd = exec.CommandContext(cfg.Ctx, cfg.ShellCommand, "-c", cfg.ShellCommandArgs)

	default:
		cmd = createDirectCommand(cfg.Ctx, cfg.Command, cfg.Args, scriptFile)

	}

	cmd.Env = append(cmd.Env, AllEnvs(ctx)...)
	cmd.Dir = cfg.Dir
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	return cmd, nil
}

func init() {
	Register("", newCommand)
	Register("command", newCommand)
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitCode int
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else {
		exitCode = 1
	}
	return exitCode
}

func newCommand(ctx context.Context, step digraph.Step) (Executor, error) {
	if len(step.Dir) > 0 && !fileutil.FileExists(step.Dir) {
		return nil, fmt.Errorf("directory does not exist: %s", step.Dir)
	}

	cfg, err := createCommandConfig(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}

	return &commandExecutor{config: cfg}, nil
}

func createCommandConfig(ctx context.Context, step digraph.Step) (*commandConfig, error) {
	shellCommand := cmdutil.GetShellCommand(step.Shell)
	shellCmdArgs := step.ShellCmdArgs

	return &commandConfig{
		Ctx:              ctx,
		Dir:              step.Dir,
		Command:          step.Command,
		Args:             step.Args,
		Script:           step.Script,
		ShellCommand:     shellCommand,
		ShellCommandArgs: shellCmdArgs,
	}, nil
}

func setupScript(ctx context.Context, step digraph.Step) (string, error) {
	file, err := os.CreateTemp(step.Dir, "dagu_script-")
	if err != nil {
		return "", fmt.Errorf("failed to create script file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	script, err := EvalString(ctx, step.Script, cmdutil.OnlyReplaceVars())
	if err != nil {
		return "", fmt.Errorf("failed to evaluate script: %w", err)
	}

	if _, err = file.WriteString(script); err != nil {
		return "", fmt.Errorf("failed to write script to file: %w", err)
	}

	if err = file.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync script file: %w", err)
	}

	return file.Name(), nil
}

// createDirectCommand creates a command that runs directly without a shell
func createDirectCommand(ctx context.Context, cmd string, args []string, scriptFile string) *exec.Cmd {
	arguments := make([]string, len(args))
	copy(arguments, args)

	if scriptFile != "" {
		arguments = append(arguments, scriptFile)
	}

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, arguments...)
}
