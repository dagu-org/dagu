package dag

import (
	"context"
	"errors"
)

// NewContext sets the current DAG to the context.
func NewContext(ctx context.Context, dag *DAG, finder DAGFinder) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{
		DAG:    dag,
		Finder: finder,
	})
}

// DAGFinder is an interface for finding a DAG by name.
type DAGFinder interface {
	FindByName(name string) (*DAG, error)
}

type Context struct {
	DAG    *DAG
	Finder DAGFinder
}

// ctxKey is used as the key for storing the DAG in the context.
type ctxKey struct{}

var (
	errFailedAssertion = errors.New("failed to assert DAG from context")
)

// GetContext returns the DAG from the current context.
func GetContext(ctx context.Context) (Context, error) {
	dag, ok := ctx.Value(ctxKey{}).(Context)
	if !ok {
		return Context{}, errFailedAssertion
	}
	return dag, nil
}
