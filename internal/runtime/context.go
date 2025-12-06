package runtime

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// Type aliases for execution package types.
// These allow runtime package users to access execution types without importing execution directly.
type (
	// Context is an alias for execution.Context
	Context = execution.Context
	// Database is an alias for execution.Database
	Database = execution.Database
	// Dispatcher is an alias for execution.Dispatcher
	Dispatcher = execution.Dispatcher
	// RunStatus is an alias for execution.RunStatus
	RunStatus = execution.RunStatus
	// ContextOption is an alias for execution.ContextOption
	ContextOption = execution.ContextOption
)

// Re-export execution package functions for convenience.
var (
	// NewContext creates a new context with DAG execution metadata.
	NewContext = execution.NewContext
	// WithDatabase sets the database interface.
	WithDatabase = execution.WithDatabase
	// WithRootDAGRun sets the root DAG run reference for sub-DAG execution.
	WithRootDAGRun = execution.WithRootDAGRun
	// WithParams sets runtime parameters.
	WithParams = execution.WithParams
	// WithCoordinator sets the coordinator dispatcher for distributed execution.
	WithCoordinator = execution.WithCoordinator
	// WithSecrets sets secret environment variables.
	WithSecrets = execution.WithSecrets
	// WithLogEncoding sets the log file character encoding.
	WithLogEncoding = execution.WithLogEncoding
)

// GetDAGContext retrieves the DAGContext from the context.
// This is a convenience wrapper for execution.GetContext.
func GetDAGContext(ctx context.Context) Context {
	return execution.GetContext(ctx)
}

// WithDAGContext returns a new context with the given DAGContext.
// This is a convenience wrapper for execution.WithContext.
func WithDAGContext(ctx context.Context, rCtx Context) context.Context {
	return execution.WithContext(ctx, rCtx)
}

// NewDAGRunRef is a convenience wrapper for execution.NewDAGRunRef.
func NewDAGRunRef(name, runID string) execution.DAGRunRef {
	return execution.NewDAGRunRef(name, runID)
}

// NewContextForTest creates a minimal context for testing purposes.
// This is useful when you need a context with just basic DAG metadata.
func NewContextForTest(ctx context.Context, dag *core.DAG, dagRunID, logFile string) context.Context {
	return execution.NewContext(ctx, dag, dagRunID, logFile)
}
