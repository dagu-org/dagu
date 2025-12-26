package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
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
	// Retrieve the SSH client stored in context by WithSSHClient
	if cli, ok := ctx.Value(sshClientCtxKey{}).(*Client); ok {
		return cli
	}
	return nil
}

type sshExecutor struct {
	mu      sync.Mutex
	step    core.Step
	client  *Client
	stdout  io.Writer
	stderr  io.Writer
	session *ssh.Session
}

func NewSSHExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	var client *Client

	// Prefer step-level SSH configuration if present
	if len(step.ExecutorConfig.Config) > 0 {
		c, err := FromMapConfig(ctx, step.ExecutorConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to setup ssh executor")
		}
		client = c
	} else if c := getSSHClientFromContext(ctx); c != nil {
		// Fall back to DAG-level SSH client from context
		client = c
	}

	if client == nil {
		return nil, fmt.Errorf("ssh configuration is not found")
	}

	return &sshExecutor{
		step:   step,
		client: client,
		stdout: os.Stdout,
		stderr: os.Stderr,
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
	session := e.session
	e.mu.Unlock()

	if session != nil {
		return session.Close()
	}
	return nil
}

func (e *sshExecutor) Run(ctx context.Context) error {
	// If no commands, nothing to execute
	if len(e.step.Commands) == 0 {
		return nil
	}

	// Execute each command sequentially
	// Each command requires a new session since session.Run can only be called once
	for i, cmdEntry := range e.step.Commands {
		// Check context cancellation between commands
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := e.runCommand(ctx, i, cmdEntry); err != nil {
			return err
		}
	}

	return nil
}

// runCommand executes a single command with context cancellation support.
// Since session.Run() blocks without context awareness, we run it in a goroutine
// and select on both completion and context cancellation for responsiveness.
func (e *sshExecutor) runCommand(ctx context.Context, index int, cmdEntry core.CommandEntry) error {
	session, err := e.client.NewSession()
	if err != nil {
		return fmt.Errorf("command %d: failed to create session: %w", index+1, err)
	}

	e.mu.Lock()
	e.session = session
	e.mu.Unlock()

	session.Stdout = e.stdout
	session.Stderr = e.stderr

	command := cmdutil.ShellQuote(cmdEntry.Command)
	if len(cmdEntry.Args) > 0 {
		command += " " + cmdutil.ShellQuoteArgs(cmdEntry.Args)
	}

	// Run command in goroutine to enable context cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	// Wait for either command completion or context cancellation
	select {
	case err = <-done:
		// Command completed (success or failure)
	case <-ctx.Done():
		// Context cancelled - close session to terminate the command
		if closeErr := session.Close(); closeErr != nil {
			logger.Warn(ctx, "SSH session close error during cancellation", tag.Error(closeErr))
		}
		return ctx.Err()
	}

	if closeErr := session.Close(); closeErr != nil {
		logger.Warn(ctx, "SSH session close error", tag.Error(closeErr))
	}

	if err != nil {
		return fmt.Errorf("command %d failed: %w", index+1, err)
	}

	return nil
}

func init() {
	caps := core.ExecutorCapabilities{
		Command:          true,
		MultipleCommands: true,
	}
	executor.RegisterExecutor("ssh", NewSSHExecutor, nil, caps)
}
