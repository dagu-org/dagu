package dag

import (
	"context"
	"errors"
)

// Finder finds a DAG by name.
// This is used to find the DAG when a node references another DAG.
type Finder interface {
	Find(name string) (*DAG, error)
}

// Context contains the current DAG and Finder.
type Context struct {
	DAG    *DAG
	Finder Finder
}

// NewContext creates a new context with the DAG and Finder.
func NewContext(ctx context.Context, dag *DAG, finder Finder) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{
		DAG:    dag,
		Finder: finder,
	})
}

// ctxKey is used as the key for storing the DAG in the context.
type ctxKey struct{}

var (
	errFailedCtxAssertion = errors.New("failed to assert DAG context")
)

// GetContext returns the DAG Context from the context.
// It returns an error if the context does not contain a DAG Context.
func GetContext(ctx context.Context) (Context, error) {
	dagCtx, ok := ctx.Value(ctxKey{}).(Context)
	if !ok {
		return Context{}, errFailedCtxAssertion
	}
	return dagCtx, nil
}
