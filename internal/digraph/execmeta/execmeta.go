package execmeta

import "context"

// Header information for executing DAGs
type ExecMeta struct {
	RootRequestID    string
	RootDAGName      string
	ParentRequestID  string
	CurrentRequestID string
}

type metaKey struct{}

func WithMeta(ctx context.Context, meta ExecMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, meta)
}

func Meta(ctx context.Context) ExecMeta {
	if meta, ok := ctx.Value(metaKey{}).(ExecMeta); ok {
		return meta
	}
	return ExecMeta{}
}
