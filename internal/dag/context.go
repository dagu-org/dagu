package dag

import (
	"context"
	"errors"
)

// NewContext sets the current DAG to the context.
func NewContext(ctx context.Context, dag *DAG) context.Context {
	return context.WithValue(ctx, DAGContextKey{}, dag)
}

// DAGContextKey is used as the key for storing the DAG in the context.
type DAGContextKey struct{}

var (
	errFailedAssertion = errors.New("failed to assert DAG from context")
)

// GetDAGFromContext returns the DAG from the current context.
func GetDAGFromContext(ctx context.Context) (*DAG, error) {
	dag, ok := ctx.Value(DAGContextKey{}).(*DAG)
	if !ok {
		return nil, errFailedAssertion
	}
	return dag, nil
}
