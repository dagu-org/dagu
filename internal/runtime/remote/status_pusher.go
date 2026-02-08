package remote

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// StatusPusher sends status updates to coordinator via gRPC
type StatusPusher struct {
	client      coordinator.Client
	workerID    string
	namespaceID string
}

// NewStatusPusher creates a new StatusPusher
func NewStatusPusher(client coordinator.Client, workerID string, namespaceID string) *StatusPusher {
	return &StatusPusher{
		client:      client,
		workerID:    workerID,
		namespaceID: namespaceID,
	}
}

// Push sends a status update to the coordinator
func (p *StatusPusher) Push(ctx context.Context, status exec.DAGRunStatus) error {
	protoStatus, err := convert.DAGRunStatusToProto(&status)
	if err != nil {
		return fmt.Errorf("failed to convert status to proto: %w", err)
	}
	req := &coordinatorv1.ReportStatusRequest{
		WorkerId:    p.workerID,
		Status:      protoStatus,
		NamespaceId: p.namespaceID,
	}

	resp, err := p.client.ReportStatus(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to report status: %w", err)
	}

	if resp == nil {
		return fmt.Errorf("received nil response from coordinator")
	}

	if !resp.Accepted {
		return fmt.Errorf("status rejected: %s", resp.Error)
	}

	return nil
}
