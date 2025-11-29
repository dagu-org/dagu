package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var _ executor.Executor = (*commandExecutor)(nil)
var _ executor.ExitCoder = (*commandExecutor)(nil)

type commandExecutor struct {
	mu         sync.Mutex
	config     *commandConfig
	cmd        *exec.Cmd
	scriptFile string
	exitCode   int
	// stderrTail stores a rolling tail of recent stderr lines
	stderrTail *executor.TailWriter
}

// ExitCode implements ExitCoder.
func (e *commandExecutor) ExitCode() int {
	return e.exitCode
}

func (e *commandExecutor) Run(ctx context.Context) error {
	e.mu.Lock()

	if e.config.Script != "" {
		scriptFile, err := setupScript(ctx, e.config.Dir, e.config.Script, e.config.ShellCommand)
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
	// Wrap stderr with a tailing writer so we can include recent
	// stderr output (rolling, up to limit) in error messages.
	tw := executor.NewTailWriter(e.config.Stderr, 0)
	e.stderrTail = tw
	e.config.Stderr = tw

	cmd, err := e.config.newCmd(ctx, e.scriptFile)
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("failed to create command: %w", err)
	}

	e.cmd = cmd

	if err := e.cmd.Start(); err != nil {
		e.exitCode = exitCodeFromError(err)
		e.mu.Unlock()
		if tail := e.stderrTail.Tail(); tail != "" {
			return fmt.Errorf("%w\nrecent stderr (tail):\n%s", err, tail)
		}
		return err
	}
	e.mu.Unlock()

	if err := e.cmd.Wait(); err != nil {
		e.exitCode = exitCodeFromError(err)
		if tail := e.stderrTail.Tail(); tail != "" {
			return fmt.Errorf("%w\nrecent stderr (tail):\n%s", err, tail)
		}
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

	return cmdutil.KillProcessGroup(e.cmd, sig)
}

type commandConfig struct {
	Ctx                context.Context
	Dir                string
	Command            string
	Args               []string
	Script             string
	ShellCommand       string
	ShellCommandArgs   string
	ShellPackages      []string
	Stdout             io.Writer
	Stderr             io.Writer
	UserSpecifiedShell bool
}

func (cfg *commandConfig) newCmd(ctx context.Context, scriptFile string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch {
	case cfg.Command != "" && scriptFile != "":
		cmdBuilder := &shellCommandBuilder{
			Command:          cfg.Command,
			Args:             cfg.Args,
			ShellCommand:     cfg.ShellCommand,
			ShellCommandArgs: cfg.ShellCommandArgs,
			ShellPackages:    cfg.ShellPackages,
			Script:           scriptFile,
		}
		c, err := cmdBuilder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	case cfg.ShellCommand != "" && scriptFile != "":
		// Check if the script has shebang and user did not specify a shell
		shebang, shebangArgs, err := cfg.detectShebang(scriptFile)
		if err != nil {
			return nil, fmt.Errorf("failed to detect shebang: %w", err)
		}
		if shebang != "" {
			// Use the shebang interpreter to run the script
			cmd = exec.CommandContext(cfg.Ctx, shebang, append(shebangArgs, scriptFile)...) // nolint: gosec
			break
		}
		// If no shebang, use the specified shell command
		command, args, err := cmdutil.SplitCommand(cfg.ShellCommand)
		if err != nil {
			return nil, fmt.Errorf("failed to parse shell command: %w", err)
		}
		args = append(args, scriptFile)
		cmd = exec.CommandContext(cfg.Ctx, command, args...) // nolint: gosec

	case cfg.ShellCommand != "" && cfg.ShellCommandArgs != "":
		cmdBuilder := &shellCommandBuilder{
			ShellCommand:     cfg.ShellCommand,
			ShellCommandArgs: cfg.ShellCommandArgs,
			ShellPackages:    cfg.ShellPackages,
		}
		c, err := cmdBuilder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	default:
		command := cfg.Command
		if command == "" {
			// If no command is specified, use the default shell.
			// Usually this should not happen.
			command = cmdutil.GetShellCommand("")
		}
		cmd = createDirectCommand(cfg.Ctx, command, cfg.Args, scriptFile)
	}

	cmd.Env = append(cmd.Env, execution.AllEnvs(ctx)...)
	cmd.Dir = cfg.Dir
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	cmdutil.SetupCommand(cmd)

	return cmd, nil
}

func (cfg *commandConfig) detectShebang(scriptFile string) (string, []string, error) {
	if cfg.UserSpecifiedShell {
		return "", nil, nil
	}
	// read the first line of the script file
	firstLine, err := readFirstLine(scriptFile)
	if err != nil {
		return "", nil, err
	}
	return cmdutil.DetectShebang(firstLine)
}

func readFirstLine(filePath string) (string, error) {
	filePath = filepath.Clean(filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	// Set a reasonable limit to prevent memory issues with extremely long lines
	// Shebangs are typically < 256 bytes, but allow up to 4KB to be safe
	const maxLineSize = 4 * 1024
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)

	if scanner.Scan() {
		return scanner.Text(), nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Empty file
	return "", nil
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

	switch cmdName {
	case "nix-shell":
		// If the shell command is nix-shell, we need to pass the packages as arguments
		for _, pkg := range b.ShellPackages {
			args = append(args, "-p", pkg)
		}
		args = append(args, "--pure")
		if !slices.Contains(args, "--run") {
			args = append(args, "--run")
		}

		if b.Command != "" && b.Script != "" {
			// When using nix-shell with a direct command and script,
			// we need to run the command inside nix-shell, not pass nix-shell args to the command
			cmdParts := []string{b.Command}
			cmdParts = append(cmdParts, b.Args...)
			cmdParts = append(cmdParts, b.Script)
			cmdStr := strings.Join(cmdParts, " ")

			// If ShellCommandArgs contains "set -e", we need to apply it
			if strings.HasPrefix(b.ShellCommandArgs, "set -e") {
				cmdStr = b.ShellCommandArgs + " " + cmdStr
			}

			return exec.CommandContext(ctx, cmd, append(args, cmdStr)...), nil // nolint: gosec
		}

		// Construct the command with the shell command and the packages
		return exec.CommandContext(ctx, b.ShellCommand, append(args, b.ShellCommandArgs)...), nil // nolint: gosec

	case "powershell.exe", "powershell":
		// PowerShell (Windows PowerShell)
		return b.buildPowerShellCommand(ctx, cmd, args)

	case "pwsh.exe", "pwsh":
		// PowerShell Core (cross-platform)
		return b.buildPowerShellCommand(ctx, cmd, args)

	case "cmd.exe", "cmd":
		// Windows Command Prompt
		return b.buildCmdCommand(ctx, cmd, args)

	default:
		// other shell (sh, bash, zsh, etc.)
		if b.Command != "" && b.Script != "" {
			// When running a command directly with a script (e.g., perl script.pl),
			// don't include shell arguments like -e
			return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
		}
		args = append(args, b.Args...)
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
	if b.Command != "" && b.Script != "" {
		// When running a command directly with a script, don't include PowerShell arguments
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	args = append(args, b.Args...)
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
	if b.Command != "" && b.Script != "" {
		// When running a command directly with a script, don't include cmd.exe arguments
		return exec.CommandContext(ctx, b.Command, append(b.Args, b.Script)...), nil // nolint: gosec
	}

	args = append(args, b.Args...)

	// cmd.exe uses /c instead of -c
	if !slices.Contains(args, "/c") && !slices.Contains(args, "/C") {
		args = append(args, "/c")
	}
	args = append(args, b.ShellCommandArgs)

	// nolint: gosec
	return exec.CommandContext(ctx, cmd, args...), nil
}

// NewCommand creates a new command executor.
func NewCommand(ctx context.Context, step core.Step) (executor.Executor, error) {
	cfg, err := NewCommandConfig(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}

	return &commandExecutor{config: cfg}, nil
}

// NewCommandConfig creates a new commandConfig from the given step.
func NewCommandConfig(ctx context.Context, step core.Step) (*commandConfig, error) {
	var shellCommand string
	shellCmdArgs := step.ShellCmdArgs
	userSpecifiedShell := step.Shell != ""

	if userSpecifiedShell {
		// User explicitly set shell - respect their choice exactly
		shellCommand = cmdutil.GetShellCommand(step.Shell)
	} else {
		// No shell specified - use default with errexit
		defaultShell := cmdutil.GetShellCommand("")

		// Special handling for nix-shell - don't add -e to nix-shell itself
		shellName := filepath.Base(defaultShell)
		if shellName == "nix-shell" {
			// For nix-shell, prepend set -e to the command
			shellCommand = defaultShell
			if shellCmdArgs != "" && !strings.HasPrefix(shellCmdArgs, "set -e") {
				shellCmdArgs = "set -e; " + shellCmdArgs
			}
		} else if isUnixLikeShell(defaultShell) {
			// Add errexit flag for Unix-like shells
			shellCommand = defaultShell + " -e"
		} else {
			shellCommand = defaultShell
		}
	}

	return &commandConfig{
		Ctx:                ctx,
		Dir:                execution.GetEnv(ctx).WorkingDir,
		Command:            step.Command,
		Args:               step.Args,
		Script:             step.Script,
		ShellCommand:       shellCommand,
		ShellCommandArgs:   shellCmdArgs,
		ShellPackages:      step.ShellPackages,
		UserSpecifiedShell: userSpecifiedShell,
	}, nil
}

// isUnixLikeShell returns true if the shell supports -e flag
func isUnixLikeShell(shell string) bool {
	if shell == "" {
		return false
	}

	// Extract just the executable name (handle full paths)
	shellName := filepath.Base(shell)

	switch shellName {
	case "sh", "bash", "zsh", "ksh", "ash", "dash":
		return true
	case "fish":
		// Fish shell doesn't support -e flag
		return false
	default:
		return false
	}
}

func setupScript(_ context.Context, workDir, script, shellCommand string) (string, error) {
	// Determine file extension based on shell
	ext := cmdutil.GetScriptExtension(shellCommand)
	pattern := "dagu_script-*" + ext

	file, err := os.CreateTemp(workDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create script file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err = file.WriteString(script); err != nil {
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

func validateCommandStep(step core.Step) error {
	switch {
	case step.Command != "" && step.Script != "":
		// Both command and script provided - valid
	case step.Command != "" && step.Script == "":
		// Command only - valid
	case step.Command == "" && step.Script != "":
		// Script only - valid
	case step.SubDAG != nil:
		// Sub DAG - valid
	default:
		return core.ErrStepCommandIsRequired
	}

	return nil
}

func init() {
	executor.RegisterExecutor("", NewCommand, validateCommandStep)
	executor.RegisterExecutor("shell", NewCommand, validateCommandStep)
	executor.RegisterExecutor("command", NewCommand, validateCommandStep)
}
