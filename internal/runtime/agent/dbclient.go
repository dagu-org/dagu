package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

var _ execution.Database = &dbClient{}

type dbClient struct {
	ds  execution.DAGStore
	drs execution.DAGRunStore
}

func newDBClient(drs execution.DAGRunStore, ds execution.DAGStore) *dbClient {
	return &dbClient{drs: drs, ds: ds}
}

// GetDAG implements core.DBClient.
func (o *dbClient) GetDAG(ctx context.Context, name string) (*core.DAG, error) {
	return o.ds.GetDetails(ctx, name)
}

func (o *dbClient) GetSubDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun execution.DAGRunRef) (*execution.RunStatus, error) {
	subAttempt, err := o.drs.FindSubAttempt(ctx, rootDAGRun, dagRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find run for dag-run ID %s: %w", dagRunID, err)
	}
	status, err := subAttempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	outputVariables := make(map[string]string)
	for _, node := range status.Nodes {
		if node.OutputVariables != nil {
			node.OutputVariables.Range(func(_, value any) bool {
				// split the value by '=' to get the key and value
				parts := strings.SplitN(value.(string), "=", 2)
				if len(parts) == 2 {
					outputVariables[parts[0]] = parts[1]
				}
				return true
			})
		}
	}

	return &execution.RunStatus{
		Status:   status.Status,
		Outputs:  outputVariables,
		Name:     status.Name,
		DAGRunID: status.DAGRunID,
		Params:   status.Params,
	}, nil
}

func (o *dbClient) IsSubDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun execution.DAGRunRef) (bool, error) {
	subAttempt, err := o.drs.FindSubAttempt(ctx, rootDAGRun, dagRunID)
	if err != nil {
		return false, fmt.Errorf("failed to find run for dag-run ID %s: %w", dagRunID, err)
	}
	status, err := subAttempt.ReadStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to read status: %w", err)
	}

	return !status.Status.IsActive(), nil
}

func (o *dbClient) RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun execution.DAGRunRef) error {
	subAttempt, err := o.drs.FindSubAttempt(ctx, rootDAGRun, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to find child attempt for dag-run ID %s: %w", dagRunID, err)
	}
	return subAttempt.RequestCancel(ctx)
}
