package runtime

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// Type aliases for execution package types.
// These allow runtime package users to access execution types without importing execution directly.
type (
	// Context is an alias for execution.Context
	Context = exec.Context
	// Database is an alias for execution.Database
	Database = exec.Database
	// Dispatcher is an alias for execution.Dispatcher
	Dispatcher = exec.Dispatcher
	// RunStatus is an alias for execution.RunStatus
	RunStatus = exec.RunStatus
	// ContextOption is an alias for execution.ContextOption
	ContextOption = exec.ContextOption
)

// Re-export execution package functions for convenience.
var (
	// NewContext creates a new context with DAG execution metadata.
	NewContext = exec.NewContext
	// WithDatabase sets the database interface.
	WithDatabase = exec.WithDatabase
	// WithRootDAGRun sets the root DAG run reference for sub-DAG execution.
	WithRootDAGRun = exec.WithRootDAGRun
	// WithParams sets runtime parameters.
	WithParams = exec.WithParams
	// WithCoordinator sets the coordinator dispatcher for distributed execution.
	WithCoordinator = exec.WithCoordinator
	// WithSecrets sets secret environment variables.
	WithSecrets = exec.WithSecrets
	// WithLogEncoding sets the log file character encoding.
	WithLogEncoding = exec.WithLogEncoding
	// WithLogWriterFactory sets the log writer factory for remote log streaming.
	WithLogWriterFactory = exec.WithLogWriterFactory
	// WithNamespace sets the active namespace for this DAG run.
	WithNamespace = exec.WithNamespace
)

// LogWriterFactory is re-exported from execution package
type LogWriterFactory = exec.LogWriterFactory

// GetDAGContext retrieves the DAGContext from the context.
// This is a convenience wrapper for execution.GetContext.
func GetDAGContext(ctx context.Context) Context {
	return exec.GetContext(ctx)
}

// WithDAGContext returns a new context with the given DAGContext.
// This is a convenience wrapper for execution.WithContext.
func WithDAGContext(ctx context.Context, rCtx Context) context.Context {
	return exec.WithContext(ctx, rCtx)
}

// NewDAGRunRef is a convenience wrapper for execution.NewDAGRunRef.
func NewDAGRunRef(name, runID string) exec.DAGRunRef {
	return exec.NewDAGRunRef(name, runID)
}

// NewContextForTest creates a minimal context for testing purposes.
// This is useful when you need a context with just basic DAG metadata.
func NewContextForTest(ctx context.Context, dag *core.DAG, dagRunID, logFile string) context.Context {
	return exec.NewContext(ctx, dag, dagRunID, logFile)
}
