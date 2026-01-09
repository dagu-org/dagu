package remote

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// StatusPusher sends status updates to coordinator via gRPC
type StatusPusher struct {
	client   coordinatorv1.CoordinatorServiceClient
	workerID string
}

// NewStatusPusher creates a new StatusPusher
func NewStatusPusher(client coordinatorv1.CoordinatorServiceClient, workerID string) *StatusPusher {
	return &StatusPusher{
		client:   client,
		workerID: workerID,
	}
}

// Push sends a status update to the coordinator
func (p *StatusPusher) Push(ctx context.Context, status execution.DAGRunStatus) error {
	req := &coordinatorv1.ReportStatusRequest{
		WorkerId: p.workerID,
		Status:   dagRunStatusToProto(&status),
	}

	resp, err := p.client.ReportStatus(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to report status: %w", err)
	}

	if !resp.Accepted {
		return fmt.Errorf("status rejected: %s", resp.Error)
	}

	return nil
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
			Name:         n.Step.Name,
			Description:  n.Step.Description,
			ExecutorType: n.Step.ExecutorConfig.Type,
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
