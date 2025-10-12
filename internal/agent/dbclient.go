package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/models"
)

var _ core.Database = &dbClient{}

type dbClient struct {
	ds  models.DAGStore
	drs models.DAGRunStore
}

func newDBClient(drs models.DAGRunStore, ds models.DAGStore) *dbClient {
	return &dbClient{drs: drs, ds: ds}
}

// GetDAG implements core.DBClient.
func (o *dbClient) GetDAG(ctx context.Context, name string) (*core.DAG, error) {
	return o.ds.GetDetails(ctx, name)
}

func (o *dbClient) GetChildDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun core.DAGRunRef) (*core.RunStatus, error) {
	childAttempt, err := o.drs.FindChildAttempt(ctx, rootDAGRun, dagRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find run for dag-run ID %s: %w", dagRunID, err)
	}
	status, err := childAttempt.ReadStatus(ctx)
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

	return &core.RunStatus{
		Status:   status.Status,
		Outputs:  outputVariables,
		Name:     status.Name,
		DAGRunID: status.DAGRunID,
		Params:   status.Params,
	}, nil
}

func (o *dbClient) IsChildDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun core.DAGRunRef) (bool, error) {
	childAttempt, err := o.drs.FindChildAttempt(ctx, rootDAGRun, dagRunID)
	if err != nil {
		return false, fmt.Errorf("failed to find run for dag-run ID %s: %w", dagRunID, err)
	}
	status, err := childAttempt.ReadStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to read status: %w", err)
	}

	return !status.Status.IsActive(), nil
}

func (o *dbClient) RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun core.DAGRunRef) error {
	childAttempt, err := o.drs.FindChildAttempt(ctx, rootDAGRun, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to find child attempt for dag-run ID %s: %w", dagRunID, err)
	}
	return childAttempt.RequestCancel(ctx)
}
