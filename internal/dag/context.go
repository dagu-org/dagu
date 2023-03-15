package dag

import "context"

// NewContext set the current DAG to the context
func NewContext(ctx context.Context, dag *DAG) context.Context {
	return context.WithValue(ctx, dagContextKey{}, dag)
}

type dagContextKey struct{}

// GetDAGFromContext returns DAG from current context
func GetDAGFromContext(ctx context.Context) *DAG {
	return ctx.Value(dagContextKey{}).(*DAG)
}
