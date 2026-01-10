// Package coordinator provides re-exports of proto conversion functions.
// The actual implementations are in the internal/proto/convert package.
package coordinator

import (
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/proto/convert"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// protoToDAGRunStatus converts a proto DAGRunStatusProto to execution.DAGRunStatus
func protoToDAGRunStatus(p *coordinatorv1.DAGRunStatusProto) *execution.DAGRunStatus {
	return convert.ProtoToDAGRunStatus(p)
}
