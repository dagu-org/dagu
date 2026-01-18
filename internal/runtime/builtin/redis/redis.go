package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var (
	_ executor.Executor = (*redisExecutor)(nil)
	_ io.Closer         = (*redisExecutor)(nil)
)

// redisExecutor implements the Executor interface for Redis operations.
type redisExecutor struct {
	mu            sync.Mutex
	step          core.Step
	cfg           *Config
	client        goredis.UniversalClient
	poolManager   *GlobalRedisPoolManager
	stdout        io.Writer
	stderr        io.Writer
	cancelFunc    context.CancelFunc
	lockRelease   func() error
	useGlobalPool bool
	closed        bool
}

// ExecutionMetrics holds metrics from Redis execution.
type ExecutionMetrics struct {
	Command    string    `json:"command,omitempty"`
	Key        string    `json:"key,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// newRedisExecutor creates a new Redis executor.
func newRedisExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	cfg, err := ParseConfig(ctx, step.ExecutorConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis config: %w", err)
	}

	// Check for global pool manager (worker mode)
	var client goredis.UniversalClient
	var poolManager *GlobalRedisPoolManager
	var useGlobalPool bool

	if pm := GetRedisPoolManager(ctx); pm != nil {
		// Use global pool for worker mode
		client, err = pm.GetOrCreateClient(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to get client from global pool: %w", err)
		}
		poolManager = pm
		useGlobalPool = true
	} else {
		// Use per-step connection (non-distributed mode)
		client, err = createClientWithRetry(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create redis client: %w", err)
		}
	}

	return &redisExecutor{
		step:          step,
		cfg:           cfg,
		client:        client,
		poolManager:   poolManager,
		useGlobalPool: useGlobalPool,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
	}, nil
}

// SetStdout sets the stdout writer.
func (e *redisExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

// SetStderr sets the stderr writer.
func (e *redisExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

// Kill cancels the execution.
func (e *redisExecutor) Kill(_ os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	return nil
}

// Close releases resources held by the Redis executor.
// In worker mode (global pool), releases the client back to the pool.
// In non-worker mode, closes the client.
// Implements io.Closer.
func (e *redisExecutor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Guard against double-close
	if e.closed {
		return nil
	}
	e.closed = true

	// Release lock if held
	if e.lockRelease != nil {
		_ = e.lockRelease()
		e.lockRelease = nil
	}

	if e.client == nil {
		return nil
	}

	var err error

	if e.useGlobalPool && e.poolManager != nil {
		// Worker mode: release client back to pool
		e.poolManager.ReleaseClient(e.cfg)
		// Don't close the client - pool owns it
	} else {
		// Non-worker mode: close the client
		err = e.client.Close()
	}

	// Clear references to prevent misuse
	e.client = nil
	e.poolManager = nil
	return err
}

// Run executes the Redis command or script.
func (e *redisExecutor) Run(ctx context.Context) error {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancelFunc = cancel
	e.mu.Unlock()
	defer cancel()

	// Apply timeout if configured
	if e.cfg.Timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(e.cfg.Timeout)*time.Second)
		defer timeoutCancel()
	}

	// Acquire lock if configured
	if e.cfg.Lock != "" {
		lockMgr := NewLockManager(e.client, e.cfg)
		release, err := lockMgr.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
		e.mu.Lock()
		e.lockRelease = release
		e.mu.Unlock()
		defer func() {
			e.mu.Lock()
			if e.lockRelease != nil {
				_ = e.lockRelease()
				e.lockRelease = nil
			}
			e.mu.Unlock()
		}()
	}

	// Determine execution mode
	if len(e.cfg.Pipeline) > 0 {
		return e.executePipeline(ctx)
	}
	if e.cfg.Script != "" || e.cfg.ScriptFile != "" {
		return e.executeScript(ctx)
	}
	return e.executeCommand(ctx)
}

// executeCommand executes a single Redis command.
func (e *redisExecutor) executeCommand(ctx context.Context) error {
	metrics := &ExecutionMetrics{
		Command:   e.cfg.Command,
		Key:       e.cfg.Key,
		StartedAt: time.Now(),
	}

	defer func() {
		metrics.FinishedAt = time.Now()
		metrics.DurationMs = metrics.FinishedAt.Sub(metrics.StartedAt).Milliseconds()
		e.writeMetrics(metrics)
	}()

	handler := NewCommandHandler(e.client, e.cfg)
	result, err := handler.Execute(ctx)
	if err != nil {
		metrics.Status = "error"
		metrics.Error = err.Error()
		return err
	}

	metrics.Status = "success"

	// Write result to stdout
	if result != nil {
		writer := NewResultWriter(e.stdout, e.cfg.OutputFormat, e.cfg.NullValue)
		if err := writer.Write(result); err != nil {
			return fmt.Errorf("failed to write result: %w", err)
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush result: %w", err)
		}
	}

	return nil
}

// executePipeline executes a pipeline of commands.
func (e *redisExecutor) executePipeline(ctx context.Context) error {
	metrics := &ExecutionMetrics{
		Command:   "PIPELINE",
		StartedAt: time.Now(),
	}

	defer func() {
		metrics.FinishedAt = time.Now()
		metrics.DurationMs = metrics.FinishedAt.Sub(metrics.StartedAt).Milliseconds()
		e.writeMetrics(metrics)
	}()

	executor := NewPipelineExecutor(e.client, e.cfg)
	results, err := executor.Execute(ctx)
	if err != nil {
		metrics.Status = "error"
		metrics.Error = err.Error()
		return err
	}

	metrics.Status = "success"

	// Write results to stdout
	writer := NewResultWriter(e.stdout, e.cfg.OutputFormat, e.cfg.NullValue)
	if err := writer.Write(results); err != nil {
		return fmt.Errorf("failed to write results: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush results: %w", err)
	}

	return nil
}

// executeScript executes a Lua script.
func (e *redisExecutor) executeScript(ctx context.Context) error {
	metrics := &ExecutionMetrics{
		Command:   "EVAL",
		StartedAt: time.Now(),
	}

	defer func() {
		metrics.FinishedAt = time.Now()
		metrics.DurationMs = metrics.FinishedAt.Sub(metrics.StartedAt).Milliseconds()
		e.writeMetrics(metrics)
	}()

	executor := NewScriptExecutor(e.client, e.cfg)
	result, err := executor.Execute(ctx)
	if err != nil {
		metrics.Status = "error"
		metrics.Error = err.Error()
		return err
	}

	metrics.Status = "success"

	// Write result to stdout
	if result != nil {
		writer := NewResultWriter(e.stdout, e.cfg.OutputFormat, e.cfg.NullValue)
		if err := writer.Write(result); err != nil {
			return fmt.Errorf("failed to write result: %w", err)
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush result: %w", err)
		}
	}

	return nil
}

// writeMetrics writes execution metrics to stderr.
func (e *redisExecutor) writeMetrics(metrics *ExecutionMetrics) {
	data, err := json.Marshal(metrics)
	if err != nil {
		return
	}
	_, _ = e.stderr.Write(data)
	_, _ = e.stderr.Write([]byte("\n"))
}

func init() {
	// Register Redis executor
	executor.RegisterExecutor(
		"redis",
		newRedisExecutor,
		nil,
		core.ExecutorCapabilities{Command: true, Script: true},
	)
}
