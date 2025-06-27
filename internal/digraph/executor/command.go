package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

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

	return killProcessGroup(e.cmd, sig)
}

type commandConfig struct {
	Ctx              context.Context
	Dir              string
	Command          string
	Args             []string
	Script           string
	ShellCommand     string
	ShellCommandArgs string
	ShellPackages    []string
	Stdout           io.Writer
	Stderr           io.Writer
}

func (cfg *commandConfig) newCmd(ctx context.Context, scriptFile string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch {
	case cfg.Command != "" && scriptFile != "":
		builder := &shellCommandBuilder{
			Command:          cfg.Command,
			Args:             cfg.Args,
			ShellCommand:     cfg.ShellCommand,
			ShellCommandArgs: cfg.ShellCommandArgs,
			ShellPackages:    cfg.ShellPackages,
			Script:           scriptFile,
		}
		c, err := builder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	case cfg.ShellCommand != "" && scriptFile != "":
		// If script is provided ignore the shell command args

		cmd = exec.CommandContext(cfg.Ctx, cfg.ShellCommand, scriptFile) // nolint: gosec

	case cfg.ShellCommand != "" && cfg.ShellCommandArgs != "":
		builder := &shellCommandBuilder{
			ShellCommand:     cfg.ShellCommand,
			ShellCommandArgs: cfg.ShellCommandArgs,
			ShellPackages:    cfg.ShellPackages,
		}
		c, err := builder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	default:
		cmd = createDirectCommand(cfg.Ctx, cfg.Command, cfg.Args, scriptFile)

	}

	cmd.Env = append(cmd.Env, AllEnvs(ctx)...)
	cmd.Dir = cfg.Dir
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	setupCommand(cmd)

	return cmd, nil
}

func init() {
	Register("", newCommand)
	Register("shell", newCommand)
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

type shellCommandBuilder struct {
	Command          string
	Args             []string
	ShellCommand     string
	ShellCommandArgs string
	ShellPackages    []string
	Script           string
}

func (b *shellCommandBuilder) Build(ctx context.Context) (*exec.Cmd, error) {
	cmd, args, err := cmdutil.SplitCommand(b.ShellCommand)
	if err != nil {
		return nil, err
	}

	// Extract just the executable name for comparison
	cmdName := strings.ToLower(filepath.Base(cmd))

	switch {
	case cmdName == "nix-shell":
		// If the shell command is nix-shell, we need to pass the packages as arguments
		for _, pkg := range b.ShellPackages {
			args = append(args, "-p", pkg)
		}
		args = append(args, "--pure")
		if !slices.Contains(args, "--run") {
			args = append(args, "--run")
		}

		if b.Command != "" && b.Script != "" {
			return exec.CommandContext(ctx, b.Command, append(args, b.Script)...), nil // nolint: gosec
		}

		// Construct the command with the shell command and the packages
		return exec.CommandContext(ctx, b.ShellCommand, append(args, b.ShellCommandArgs)...), nil // nolint: gosec

	case cmdName == "powershell.exe" || cmdName == "powershell":
		// PowerShell (Windows PowerShell)
		return b.buildPowerShellCommand(ctx, cmd, args)

	case cmdName == "pwsh.exe" || cmdName == "pwsh":
		// PowerShell Core (cross-platform)
		return b.buildPowerShellCommand(ctx, cmd, args)

	case cmdName == "cmd.exe" || cmdName == "cmd":
		// Windows Command Prompt
		return b.buildCmdCommand(ctx, cmd, args)

	default:
		// other shell (sh, bash, zsh, etc.)
		args = append(args, b.Args...)
		if b.Command != "" && b.Script != "" {
			return exec.CommandContext(ctx, b.Command, append(args, b.Script)...), nil // nolint: gosec
		}
		if !slices.Contains(args, "-c") {
			args = append(args, "-c")
		}
		args = append(args, b.ShellCommandArgs)

		// nolint: gosec
		return exec.CommandContext(ctx, cmd, args...), nil
	}
}

// buildPowerShellCommand builds a command for PowerShell (both Windows PowerShell and PowerShell Core)
func (b *shellCommandBuilder) buildPowerShellCommand(ctx context.Context, cmd string, args []string) (*exec.Cmd, error) {
	args = append(args, b.Args...)
	
	if b.Command != "" && b.Script != "" {
		return exec.CommandContext(ctx, b.Command, append(args, b.Script)...), nil // nolint: gosec
	}
	
	// PowerShell uses -Command instead of -c
	if !slices.Contains(args, "-Command") && !slices.Contains(args, "-C") {
		args = append(args, "-Command")
	}
	args = append(args, b.ShellCommandArgs)

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, args...), nil
}

// buildCmdCommand builds a command for Windows cmd.exe
func (b *shellCommandBuilder) buildCmdCommand(ctx context.Context, cmd string, args []string) (*exec.Cmd, error) {
	args = append(args, b.Args...)
	
	if b.Command != "" && b.Script != "" {
		return exec.CommandContext(ctx, b.Command, append(args, b.Script)...), nil // nolint: gosec
	}
	
	// cmd.exe uses /c instead of -c
	if !slices.Contains(args, "/c") && !slices.Contains(args, "/C") {
		args = append(args, "/c")
	}
	args = append(args, b.ShellCommandArgs)

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, args...), nil
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
		ShellPackages:    step.ShellPackages,
	}, nil
}

func setupScript(_ context.Context, step digraph.Step) (string, error) {
	file, err := os.CreateTemp(step.Dir, "dagu_script-")
	if err != nil {
		return "", fmt.Errorf("failed to create script file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err = file.WriteString(step.Script); err != nil {
		return "", fmt.Errorf("failed to write script to file: %w", err)
	}

	if err = file.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync script file: %w", err)
	}

	// Add execute permissions to the script file
	if err = os.Chmod(file.Name(), 0750); err != nil { // nolint: gosec
		return "", fmt.Errorf("failed to set execute permissions on script file: %w", err)
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
