// Package convert provides conversion functions between execution types and proto messages.
package convert

import (
	"encoding/json"

	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// DAGRunStatusToProto converts execution.DAGRunStatus to proto.
func DAGRunStatusToProto(s *execution.DAGRunStatus) *coordinatorv1.DAGRunStatusProto {
	if s == nil {
		return nil
	}
	data, _ := json.Marshal(s)
	return &coordinatorv1.DAGRunStatusProto{JsonData: string(data)}
}

// ProtoToDAGRunStatus converts proto to execution.DAGRunStatus.
func ProtoToDAGRunStatus(p *coordinatorv1.DAGRunStatusProto) *execution.DAGRunStatus {
	if p == nil || p.JsonData == "" {
		return nil
	}
	var s execution.DAGRunStatus
	_ = json.Unmarshal([]byte(p.JsonData), &s)
	return &s
}
