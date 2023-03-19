package dag

import "context"

// NewContext sets the current DAG to the context.
func NewContext(ctx context.Context, dag *DAG) context.Context {
	return context.WithValue(ctx, DAGContextKey{}, dag)
}

// DAGContextKey is used as the key for storing the DAG in the context.
type DAGContextKey struct{}

// GetDAGFromContext returns the DAG from the current context.
func GetDAGFromContext(ctx context.Context) *DAG {
	return ctx.Value(DAGContextKey{}).(*DAG)
}
