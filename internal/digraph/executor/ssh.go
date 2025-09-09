package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/sshutil"
)

var _ Executor = (*sshExec)(nil)

type sshClientCtxKey = struct{}

// WithSSHClient creates a new context with sshutil.client
func WithSSHClient(ctx context.Context, cli *sshutil.Client) context.Context {
	return context.WithValue(ctx, sshClientCtxKey{}, cli)
}

// getSSHClientFromContext retrieves the sshutil.Client from the context.
func getSSHClientFromContext(ctx context.Context) *sshutil.Client {
	// Retrieve the SSH client stored in context by WithSSHClient
	if cli, ok := ctx.Value(sshClientCtxKey{}).(*sshutil.Client); ok {
		return cli
	}
	return nil
}

type sshExec struct {
	step    digraph.Step
	client  *sshutil.Client
	stdout  io.Writer
	stderr  io.Writer
	session *ssh.Session
}

func newSSHExec(ctx context.Context, step digraph.Step) (Executor, error) {
	var client *sshutil.Client

	// Prefer step-level SSH configuration if present
	if len(step.ExecutorConfig.Config) > 0 {
		c, err := sshutil.FromMapConfig(ctx, step.ExecutorConfig.Config)
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

	return &sshExec{
		step:   step,
		client: client,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}, nil
}

func (e *sshExec) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *sshExec) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *sshExec) Kill(_ os.Signal) error {
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}

func (e *sshExec) Run(_ context.Context) error {
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

func init() {
	Register("ssh", newSSHExec)
}
