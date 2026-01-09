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

// protoToDAGRunRef converts a proto DAGRunRefProto to execution.DAGRunRef
func protoToDAGRunRef(p *coordinatorv1.DAGRunRefProto) execution.DAGRunRef {
	return convert.ProtoToDAGRunRef(p)
}

// protoToNode converts a proto NodeStatusProto to execution.Node
func protoToNode(p *coordinatorv1.NodeStatusProto) *execution.Node {
	return convert.ProtoToNode(p)
}

// dagRunStatusToProto converts execution.DAGRunStatus to proto DAGRunStatusProto
func dagRunStatusToProto(s *execution.DAGRunStatus) *coordinatorv1.DAGRunStatusProto {
	return convert.DAGRunStatusToProto(s)
}

// dagRunRefToProto converts execution.DAGRunRef to proto DAGRunRefProto
func dagRunRefToProto(r execution.DAGRunRef) *coordinatorv1.DAGRunRefProto {
	return convert.DAGRunRefToProto(r)
}

// nodeToProto converts execution.Node to proto NodeStatusProto
func nodeToProto(n *execution.Node) *coordinatorv1.NodeStatusProto {
	return convert.NodeToProto(n)
}
