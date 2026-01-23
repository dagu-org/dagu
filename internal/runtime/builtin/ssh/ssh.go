package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"golang.org/x/crypto/ssh"
)

var _ executor.Executor = (*sshExecutor)(nil)

type sshClientCtxKey struct{}

// WithSSHClient creates a new context with client
func WithSSHClient(ctx context.Context, cli *Client) context.Context {
	return context.WithValue(ctx, sshClientCtxKey{}, cli)
}

// getSSHClientFromContext retrieves the Client from the context.
func getSSHClientFromContext(ctx context.Context) *Client {
	if cli, ok := ctx.Value(sshClientCtxKey{}).(*Client); ok {
		return cli
	}
	return nil
}

type sshExecutor struct {
	mu        sync.Mutex
	step      core.Step
	client    *Client
	stdout    io.Writer
	stderr    io.Writer
	conn      *ssh.Client  // SSH connection (must be closed after session)
	session   *ssh.Session // SSH session
	closed    bool         // Whether session/conn have been closed
	shell     string
	shellArgs []string
	envVars   map[string]string // Environment variables to export on remote
}

func NewSSHExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	var client *Client

	// Prefer step-level SSH configuration if present
	if len(step.ExecutorConfig.Config) > 0 {
		c, err := FromMapConfig(ctx, step.ExecutorConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to setup ssh executor: %w", err)
		}
		client = c
	} else if c := getSSHClientFromContext(ctx); c != nil {
		// Fall back to DAG-level SSH client from context
		client = c
	}

	if client == nil {
		return nil, fmt.Errorf("ssh configuration is not found")
	}

	shell, shellArgs := resolveShell(step, client)

	// Merge environment variables: DAG-level (from client) + step-level
	// Step-level env vars override DAG-level for the same key
	envVars := mergeEnvVars(client.Env, parseEnvSlice(step.Env))

	return &sshExecutor{
		step:      step,
		client:    client,
		shell:     shell,
		shellArgs: shellArgs,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		envVars:   envVars,
	}, nil
}

func (e *sshExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *sshExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *sshExecutor) Kill(_ os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	var lastErr error
	if e.session != nil {
		lastErr = e.session.Close()
	}
	if e.conn != nil {
		if err := e.conn.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (e *sshExecutor) Run(ctx context.Context) error {
	if len(e.step.Commands) == 0 && e.step.Script == "" {
		return nil
	}

	conn, session, err := e.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}

	e.mu.Lock()
	e.conn = conn
	e.session = session
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()

		if e.closed {
			return
		}

		// Close session first, then the underlying connection
		if closeErr := session.Close(); closeErr != nil {
			logger.Warn(ctx, "SSH session close error", tag.Error(closeErr))
		}
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn(ctx, "SSH connection close error", tag.Error(closeErr))
		}
		e.closed = true
	}()

	session.Stdout = e.stdout
	session.Stderr = e.stderr
	session.Stdin = strings.NewReader(e.buildScript())

	return e.runWithCancellation(ctx, session, e.buildShellCommand())
}

// runWithCancellation executes the session command with context cancellation support.
func (e *sshExecutor) runWithCancellation(ctx context.Context, session *ssh.Session, shellCmd string) error {
	done := make(chan error, 1)
	go func() {
		done <- session.Run(shellCmd)
	}()

	select {
	case err := <-done:
		if err == nil {
			return nil
		}
		return fmt.Errorf("ssh execution failed: %w", err)
	case <-ctx.Done():
		// Close session to unblock the goroutine, then wait for it to finish
		_ = session.Close()
		<-done
		return ctx.Err()
	}
}

// buildShellCommand constructs the shell command string with arguments.
func (e *sshExecutor) buildShellCommand() string {
	if len(e.shellArgs) == 0 {
		return e.shell
	}
	return e.shell + " " + strings.Join(e.shellArgs, " ")
}

// buildScript constructs a complete script for SSH execution, wrapped in a function.
// The function wrapper ensures the shell reads all input before execution,
// making it robust against slow networks and buffering issues.
func (e *sshExecutor) buildScript() string {
	var body strings.Builder

	// Export environment variables first (before cd or any commands)
	// This ensures env vars are available for all subsequent commands
	for key, value := range e.envVars {
		fmt.Fprintf(&body, "export %s=%s\n", key, cmdutil.ShellQuote(value))
	}

	// For SSH execution, only use working directory if explicitly set at step level.
	// DAG-level workingDir is for LOCAL execution and may not exist on the remote host.
	// If step.Dir is empty, run in SSH user's home directory.
	workingDir := e.step.Dir

	// Change to working directory if explicitly specified
	if workingDir != "" {
		fmt.Fprintf(&body, "cd %s || return 1\n", cmdutil.ShellQuote(workingDir))
	}

	// Add error handling (exit on first error)
	body.WriteString("set -e\n")

	// Add script content or commands
	if e.step.Script != "" {
		body.WriteString(e.step.Script)
		if !strings.HasSuffix(e.step.Script, "\n") {
			body.WriteString("\n")
		}
	} else {
		for _, cmd := range e.step.Commands {
			body.WriteString(e.buildCommandString(cmd))
			body.WriteString("\n")
		}
	}

	// Wrap in function - shell MUST read entire body before executing
	return fmt.Sprintf("__dagu_exec(){\n%s}\n__dagu_exec\n", body.String())
}

// mergeEnvVars merges base and override environment variable maps.
// Override values take precedence for keys that exist in both maps.
func mergeEnvVars(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	result := make(map[string]string, len(base)+len(override))

	// Copy base first
	for k, v := range base {
		result[k] = v
	}

	// Override with step-level values
	for k, v := range override {
		result[k] = v
	}

	return result
}

// parseEnvSlice converts a slice of "KEY=VALUE" strings to a map.
func parseEnvSlice(envs []string) map[string]string {
	if len(envs) == 0 {
		return nil
	}

	result := make(map[string]string, len(envs))
	for _, env := range envs {
		key, value, found := strings.Cut(env, "=")
		if found {
			result[key] = value
		}
	}

	return result
}

// buildCommandString constructs a simple command string from a CommandEntry.
func (e *sshExecutor) buildCommandString(cmd core.CommandEntry) string {
	if len(cmd.Args) == 0 {
		return cmd.Command
	}
	return cmd.Command + " " + cmdutil.ShellQuoteArgs(cmd.Args)
}

// resolveShell determines the shell to use for remote execution.
// Priority:
// 1. Shell specified in SSH configuration (step-level or DAG-level).
// 2. Shell specified in the step's Shell field.
// 3. /bin/sh as POSIX-compliant fallback.
// Note: DAG-level shell (dag.Shell) is NOT used as it's configured for local execution.
func resolveShell(step core.Step, client *Client) (string, []string) {
	if client != nil && client.Shell != "" {
		return client.Shell, slices.Clone(client.ShellArgs)
	}
	if step.Shell != "" {
		return step.Shell, slices.Clone(step.ShellArgs)
	}
	// Fallback to /bin/sh - POSIX standard, available on all Unix systems
	return "/bin/sh", nil
}

func init() {
	caps := core.ExecutorCapabilities{
		Command:          true,
		MultipleCommands: true,
		Script:           true,
		Shell:            true,
		GetEvalOptions: func(ctx context.Context, step core.Step) []cmdutil.EvalOption {
			if hasShellConfigured(ctx, step) {
				// Shell is configured, shell features (expansion, pipes, etc.) are supported
				return nil
			}
			// No shell configured - skip shell expansion for remote execution
			return []cmdutil.EvalOption{cmdutil.WithoutExpandShell()}
		},
	}
	executor.RegisterExecutor("ssh", NewSSHExecutor, nil, caps)
}

func hasShellConfigured(ctx context.Context, step core.Step) bool {
	if len(step.ExecutorConfig.Config) > 0 {
		shellValue, ok := step.ExecutorConfig.Config["shell"]
		if ok {
			switch v := shellValue.(type) {
			case string:
				return strings.TrimSpace(v) != ""
			case []any:
				return len(v) > 0
			case []string:
				return len(v) > 0
			}
		}
		return false
	}

	if cli := getSSHClientFromContext(ctx); cli != nil && cli.Shell != "" {
		return true
	}

	return step.Shell != ""
}
