package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
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
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}

func (e *sshExecutor) Run(_ context.Context) error {
	session, err := e.client.NewSession()
	if err != nil {
		return err
	}
	e.session = session
	defer func() {
		_ = session.Close()
	}()

	// Once a Session is created, you can execute a single command on
	// the remote side using the Run method.
	session.Stdout = e.stdout
	session.Stderr = e.stderr
	command := strings.Join(
		append([]string{e.step.Command}, e.step.Args...), " ",
	)
	return session.Run(command)
}

// ValidateStep implements StepValidator interface for SSH executor.
// SSH executor does not support the script field, only command field.
func (e *sshExecutor) ValidateStep(step *core.Step) error {
	if step.Script != "" {
		return fmt.Errorf(
			"script field is not supported with SSH executor. " +
				"Use 'command' field instead. " +
				"See: https://github.com/dagu-org/dagu/issues/1306",
		)
	}
	return nil
}

func validateSSHStep(step core.Step) error {
	if step.Script != "" {
		return fmt.Errorf(
			"script field is not supported with SSH executor. " +
				"Use 'command' field instead. " +
				"See: https://github.com/dagu-org/dagu/issues/1306",
		)
	}
	return nil
}

func init() {
	executor.RegisterExecutor("ssh", NewSSHExecutor, validateSSHStep)
}
