package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

const (
	executorType = "s3"

	opUpload   = "upload"
	opDownload = "download"
	opList     = "list"
	opDelete   = "delete"
)

var (
	_ executor.Executor  = (*executorImpl)(nil)
	_ executor.ExitCoder = (*executorImpl)(nil)
	_ io.Closer          = (*executorImpl)(nil)
)

func init() {
	executor.RegisterExecutor(executorType, newExecutor, validateStep, core.ExecutorCapabilities{Command: true})
}

func validateStep(step core.Step) error {
	if step.ExecutorConfig.Type != executorType {
		return nil
	}
	// Check for command in Commands field
	var command string
	if len(step.Commands) > 0 {
		command = step.Commands[0].Command
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("%w: command is required (upload, download, list, delete)", ErrConfig)
	}
	return nil
}

type executorImpl struct {
	mu       sync.Mutex
	stdout   io.Writer
	stderr   io.Writer
	cancel   context.CancelFunc
	ctx      context.Context
	exitCode int

	cfg       *Config
	operation string
	client    *s3.Client
	closed    bool
}

func newExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	cfg := DefaultConfig()

	// Get DAG-level S3 config from context and apply as defaults
	if dagS3 := getS3ConfigFromContext(ctx); dagS3 != nil {
		cfg.ApplyDefaults(dagS3)
	}

	// Step-level config overrides DAG-level defaults
	if step.ExecutorConfig.Config != nil {
		if err := decodeConfig(step.ExecutorConfig.Config, cfg); err != nil {
			return nil, fmt.Errorf("invalid s3 configuration: %w", err)
		}
	}

	// Extract command from Commands field
	var command string
	if len(step.Commands) > 0 {
		command = step.Commands[0].Command
	}
	op := strings.ToLower(strings.TrimSpace(command))
	if op == "" {
		return nil, fmt.Errorf("%w: command must specify s3 operation", ErrConfig)
	}

	switch op {
	case opUpload, opDownload, opList, opDelete:
	default:
		return nil, fmt.Errorf("%w: unsupported s3 operation %q (valid: upload, download, list, delete)", ErrConfig, op)
	}

	if err := cfg.ValidateForOperation(op); err != nil {
		return nil, err
	}

	rCtx, cancel := context.WithCancel(ctx)

	// Create AWS S3 client
	client, err := createClient(rCtx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	return &executorImpl{
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		cancel:    cancel,
		ctx:       rCtx,
		cfg:       cfg,
		operation: op,
		client:    client,
	}, nil
}

func (e *executorImpl) SetStdout(out io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stdout = out
}

func (e *executorImpl) SetStderr(out io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stderr = out
}

func (e *executorImpl) Run(_ context.Context) error {
	e.mu.Lock()
	ctx := e.ctx
	e.mu.Unlock()

	var err error

	switch e.operation {
	case opUpload:
		err = e.runUpload(ctx)
	case opDownload:
		err = e.runDownload(ctx)
	case opList:
		err = e.runList(ctx)
	case opDelete:
		err = e.runDelete(ctx)
	default:
		err = fmt.Errorf("%w: unknown operation %q", ErrConfig, e.operation)
	}

	e.mu.Lock()
	if err == nil {
		e.exitCode = 0
	} else {
		e.exitCode = exitCodeFor(err)
	}
	e.mu.Unlock()

	return err
}

func (e *executorImpl) Kill(_ os.Signal) error {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (e *executorImpl) ExitCode() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exitCode
}

func (e *executorImpl) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	// AWS SDK v2 clients don't need explicit close
	return nil
}

// writeResult writes the result as JSON to stdout.
func (e *executorImpl) writeResult(v any) error {
	return encodeJSON(e.stdout, v)
}

// logError writes an error message to stderr.
func (e *executorImpl) logError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(e.stderr, "s3: %s\n", msg)
}
