package coordinator

import (
	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// protoToDAGRunStatus converts a proto DAGRunStatusProto to execution.DAGRunStatus
func protoToDAGRunStatus(p *coordinatorv1.DAGRunStatusProto) *execution.DAGRunStatus {
	if p == nil {
		return nil
	}

	status := &execution.DAGRunStatus{
		Root:       protoToDAGRunRef(p.Root),
		Parent:     protoToDAGRunRef(p.Parent),
		Name:       p.Name,
		DAGRunID:   p.DagRunId,
		AttemptID:  p.AttemptId,
		Status:     core.Status(p.Status),
		WorkerID:   p.WorkerId,
		PID:        execution.PID(p.Pid),
		CreatedAt:  p.CreatedAt,
		QueuedAt:   p.QueuedAt,
		StartedAt:  p.StartedAt,
		FinishedAt: p.FinishedAt,
		Log:        p.Log,
		Error:      p.Error,
		Params:     p.Params,
		ParamsList: p.ParamsList,
	}

	// Convert nodes
	status.Nodes = make([]*execution.Node, len(p.Nodes))
	for i, n := range p.Nodes {
		status.Nodes[i] = protoToNode(n)
	}

	// Convert handler nodes
	status.OnInit = protoToNode(p.OnInit)
	status.OnExit = protoToNode(p.OnExit)
	status.OnSuccess = protoToNode(p.OnSuccess)
	status.OnFailure = protoToNode(p.OnFailure)
	status.OnCancel = protoToNode(p.OnCancel)
	status.OnWait = protoToNode(p.OnWait)

	return status
}

// protoToDAGRunRef converts a proto DAGRunRefProto to execution.DAGRunRef
func protoToDAGRunRef(p *coordinatorv1.DAGRunRefProto) execution.DAGRunRef {
	if p == nil {
		return execution.DAGRunRef{}
	}
	return execution.DAGRunRef{
		Name: p.Name,
		ID:   p.Id,
	}
}

// protoToNode converts a proto NodeStatusProto to execution.Node
func protoToNode(p *coordinatorv1.NodeStatusProto) *execution.Node {
	if p == nil {
		return nil
	}

	node := &execution.Node{
		Step: core.Step{
			Name: p.StepName,
		},
		Stdout:     p.Stdout,
		Stderr:     p.Stderr,
		StartedAt:  p.StartedAt,
		FinishedAt: p.FinishedAt,
		Status:     core.NodeStatus(p.Status),
		RetriedAt:  p.RetriedAt,
		RetryCount: int(p.RetryCount),
		DoneCount:  int(p.DoneCount),
		Error:      p.Error,
	}

	// Convert step info if present
	if p.Step != nil {
		node.Step.Name = p.Step.Name
		node.Step.Description = p.Step.Description
	}

	// Convert sub-runs
	node.SubRuns = make([]execution.SubDAGRun, len(p.SubRuns))
	for i, sr := range p.SubRuns {
		node.SubRuns[i] = execution.SubDAGRun{
			DAGRunID: sr.DagRunId,
			Params:   sr.Params,
		}
	}

	// Convert output variables
	if len(p.OutputVariables) > 0 {
		node.OutputVariables = &collections.SyncMap{}
		for k, v := range p.OutputVariables {
			node.OutputVariables.Store(k, v)
		}
	}

	return node
}

// dagRunStatusToProto converts execution.DAGRunStatus to proto DAGRunStatusProto
func dagRunStatusToProto(s *execution.DAGRunStatus) *coordinatorv1.DAGRunStatusProto {
	if s == nil {
		return nil
	}

	p := &coordinatorv1.DAGRunStatusProto{
		Root:       dagRunRefToProto(s.Root),
		Parent:     dagRunRefToProto(s.Parent),
		Name:       s.Name,
		DagRunId:   s.DAGRunID,
		AttemptId:  s.AttemptID,
		Status:     int32(s.Status),
		WorkerId:   s.WorkerID,
		Pid:        int32(s.PID),
		CreatedAt:  s.CreatedAt,
		QueuedAt:   s.QueuedAt,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Log:        s.Log,
		Error:      s.Error,
		Params:     s.Params,
		ParamsList: s.ParamsList,
	}

	// Convert nodes
	p.Nodes = make([]*coordinatorv1.NodeStatusProto, len(s.Nodes))
	for i, n := range s.Nodes {
		p.Nodes[i] = nodeToProto(n)
	}

	// Convert handler nodes
	p.OnInit = nodeToProto(s.OnInit)
	p.OnExit = nodeToProto(s.OnExit)
	p.OnSuccess = nodeToProto(s.OnSuccess)
	p.OnFailure = nodeToProto(s.OnFailure)
	p.OnCancel = nodeToProto(s.OnCancel)
	p.OnWait = nodeToProto(s.OnWait)

	return p
}

// dagRunRefToProto converts execution.DAGRunRef to proto DAGRunRefProto
func dagRunRefToProto(r execution.DAGRunRef) *coordinatorv1.DAGRunRefProto {
	if r.Zero() {
		return nil
	}
	return &coordinatorv1.DAGRunRefProto{
		Name: r.Name,
		Id:   r.ID,
	}
}

// nodeToProto converts execution.Node to proto NodeStatusProto
func nodeToProto(n *execution.Node) *coordinatorv1.NodeStatusProto {
	if n == nil {
		return nil
	}

	p := &coordinatorv1.NodeStatusProto{
		StepName:   n.Step.Name,
		Status:     int32(n.Status),
		Stdout:     n.Stdout,
		Stderr:     n.Stderr,
		StartedAt:  n.StartedAt,
		FinishedAt: n.FinishedAt,
		Error:      n.Error,
		RetryCount: int32(n.RetryCount),
		DoneCount:  int32(n.DoneCount),
		RetriedAt:  n.RetriedAt,
		Step: &coordinatorv1.StepProto{
			Name:        n.Step.Name,
			Description: n.Step.Description,
		},
	}

	// Convert sub-runs
	p.SubRuns = make([]*coordinatorv1.SubDAGRunProto, len(n.SubRuns))
	for i, sr := range n.SubRuns {
		p.SubRuns[i] = &coordinatorv1.SubDAGRunProto{
			DagRunId: sr.DAGRunID,
			Params:   sr.Params,
		}
	}

	// Convert output variables
	if n.OutputVariables != nil {
		p.OutputVariables = make(map[string]string)
		n.OutputVariables.Range(func(key, value interface{}) bool {
			if k, ok := key.(string); ok {
				if v, ok := value.(string); ok {
					p.OutputVariables[k] = v
				}
			}
			return true
		})
	}

	return p
}
