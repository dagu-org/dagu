package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var errNoCommandSpecified = fmt.Errorf("no command specified")

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
		scriptFile, err := setupScript(e.config.Dir, e.config.Script, e.config.Shell)
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
	// Use encoding from DAGContext to properly decode non-UTF-8 output.
	env := runtime.GetEnv(ctx)
	tw := executor.NewTailWriterWithEncoding(e.config.Stderr, 0, env.LogEncodingCharset)
	e.stderrTail = tw
	e.config.Stderr = tw

	cmd, err := e.config.newCmd(ctx, e.scriptFile)
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("failed to create command: %w", err)
	}

	e.cmd = cmd

	// Ensure the working directory exists
	if cmd.Dir != "" {
		if err := os.MkdirAll(cmd.Dir, 0750); err != nil {
			e.mu.Unlock()
			return fmt.Errorf("failed to create working directory: %w", err)
		}
	}

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
	Shell              []string // Shell command and arguments, e.g., ["/bin/sh", "-e"]
	ShellCommandArgs   string   // The command string to execute via shell -c
	ShellPackages      []string // Packages for nix-shell
	Stdout             io.Writer
	Stderr             io.Writer
	UserSpecifiedShell bool
}

func (cfg *commandConfig) newCmd(ctx context.Context, scriptFile string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch {
	case cfg.Command != "" && scriptFile != "":
		cmdBuilder := &shellCommandBuilder{
			Dir:                cfg.Dir,
			Command:            cfg.Command,
			Args:               cfg.Args,
			Shell:              cfg.Shell,
			ShellCommandArgs:   cfg.ShellCommandArgs,
			ShellPackages:      cfg.ShellPackages,
			Script:             scriptFile,
			UserSpecifiedShell: cfg.UserSpecifiedShell,
		}
		c, err := cmdBuilder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	case len(cfg.Shell) > 0 && scriptFile != "":
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
		// Use shell command builder to properly execute the script file
		// This ensures each shell type uses the correct flags (e.g., cmd.exe /c, powershell -File)
		cmdBuilder := &shellCommandBuilder{
			Dir:                cfg.Dir,
			Shell:              cfg.Shell,
			Script:             scriptFile,
			UserSpecifiedShell: cfg.UserSpecifiedShell,
		}
		c, err := cmdBuilder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	case len(cfg.Shell) > 0 && cfg.ShellCommandArgs != "":
		cmdBuilder := &shellCommandBuilder{
			Dir:                cfg.Dir,
			Command:            cfg.Command,
			Args:               cfg.Args,
			Shell:              cfg.Shell,
			ShellCommandArgs:   cfg.ShellCommandArgs,
			ShellPackages:      cfg.ShellPackages,
			UserSpecifiedShell: cfg.UserSpecifiedShell,
		}
		c, err := cmdBuilder.Build(ctx)
		if err != nil {
			return nil, err
		}
		cmd = c

	default:
		command := cfg.Command
		args := cfg.Args
		if command == "" {
			// No command specified, fallback to shell
			env := runtime.GetEnv(ctx)
			shell := env.Shell(ctx)
			if len(shell) == 0 {
				return nil, errNoCommandSpecified
			}
			command = shell[0]
			tmp := make([]string, len(shell)-1)
			copy(tmp, shell[1:])
			args = append(tmp, args...)
		}
		cmd = createDirectCommand(cfg.Ctx, command, args, scriptFile)
	}

	cmd.Env = append(cmd.Env, runtime.AllEnvs(ctx)...)
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

// exitCodeFromError returns the process exit code represented by err.
// 0 if err is nil; if err is an *exec.ExitError (or wraps one) returns its ExitCode(); otherwise returns 1.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

// NewCommand creates an executor that will run the provided step.
// It returns an executor configured from the step, or an error if creating the command configuration fails.
// If the step has multiple commands, it returns a multiCommandExecutor that runs them sequentially.
func NewCommand(ctx context.Context, step core.Step) (executor.Executor, error) {
	// If there are multiple commands, use the multi-command executor
	if len(step.Commands) > 1 {
		return newMultiCommandExecutor(ctx, step)
	}

	cfg, err := NewCommandConfig(ctx, step)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}

	return &commandExecutor{config: cfg}, nil
}

// NewCommandConfig creates a commandConfig populated from the given context and step.
// The returned config uses the environment from runtime.GetEnv(ctx) for Dir and Shell,
// extracts Command/Args from the first command entry, and sets UserSpecifiedShell to true
// when the step explicitly provided a Shell.
// It returns the constructed *commandConfig and a nil error.
func NewCommandConfig(ctx context.Context, step core.Step) (*commandConfig, error) {
	env := runtime.GetEnv(ctx)

	var command string
	var args []string
	var shellCmdArgs string

	// Extract command and args from the first command entry
	if len(step.Commands) > 0 {
		command = step.Commands[0].Command
		args = step.Commands[0].Args
		shellCmdArgs = step.Commands[0].CmdWithArgs
	}

	// Fall back to step-level ShellCmdArgs if not set in command entry
	if shellCmdArgs == "" {
		shellCmdArgs = step.ShellCmdArgs
	}

	return &commandConfig{
		Ctx:                ctx,
		Dir:                env.WorkingDir,
		Command:            command,
		Args:               args,
		Script:             step.Script,
		Shell:              env.Shell(ctx),
		ShellCommandArgs:   shellCmdArgs,
		ShellPackages:      step.ShellPackages,
		UserSpecifiedShell: step.Shell != "",
	}, nil
}

// init registers command executors ("", "shell", "command") with the executor
// framework, associating each with NewCommand and validateCommandStep.
func init() {
	caps := core.ExecutorCapabilities{
		Command:          true,
		MultipleCommands: true,
		Script:           true,
		Shell:            true,
		GetEvalOptions: func(ctx context.Context, step core.Step) []cmdutil.EvalOption {
			env := runtime.GetEnv(ctx)
			if len(env.Shell(ctx)) > 0 {
				// Shell will handle env expansion
				return []cmdutil.EvalOption{cmdutil.WithoutExpandEnv()}
			}
			return nil
		},
	}
	executor.RegisterExecutor("", NewCommand, validateCommandStep, caps)
	executor.RegisterExecutor("shell", NewCommand, validateCommandStep, caps)
	executor.RegisterExecutor("command", NewCommand, validateCommandStep, caps)
}
