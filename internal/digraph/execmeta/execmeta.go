package execmeta

import "context"

// Header information for executing DAGs
type ExecMeta struct {
	RootReqID    string
	RootDAGName  string
	ParentReqID  string
	CurrentReqID string
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
