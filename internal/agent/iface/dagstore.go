package iface

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
)

// DAGMetadataStore resolves DAG metadata used by the agent API.
type DAGMetadataStore interface {
	GetMetadata(ctx context.Context, fileName string) (*core.DAG, error)
}
