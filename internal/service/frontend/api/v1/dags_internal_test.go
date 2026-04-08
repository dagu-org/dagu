// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	openapi "github.com/dagucloud/dagu/api/v1"
	localapi "github.com/dagucloud/dagu/internal/service/frontend/api/v1"
	"github.com/dagucloud/dagu/internal/service/scheduler"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

type stubSchedulerStateStore struct {
	state *scheduler.SchedulerState
}

func (s stubSchedulerStateStore) Load(context.Context) (*scheduler.SchedulerState, error) {
	return s.state, nil
}

func (stubSchedulerStateStore) Save(context.Context, *scheduler.SchedulerState) error {
	return nil
}

func TestListDAGsDataPreservesNextRunAcrossSSEPath(t *testing.T) {
	t.Parallel()

	helper := test.Setup(t, test.WithStatusPersistence())
	scheduledAt := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	dag := helper.DAG(t, fmt.Sprintf(`
name: sse-next-run-dag
schedule:
  - at: "%s"
steps:
  - command: echo hi
`, scheduledAt.Format(time.RFC3339)))

	state := &scheduler.SchedulerState{
		Version: scheduler.SchedulerStateVersion,
		DAGs: map[string]scheduler.DAGWatermark{
			dag.Name: {
				OneOffs: map[string]scheduler.OneOffScheduleState{
					dag.Schedule[0].Fingerprint(): {
						ScheduledTime: scheduledAt,
						Status:        scheduler.OneOffStatusPending,
					},
				},
			},
		},
	}

	api := localapi.New(
		helper.DAGStore,
		helper.DAGRunStore,
		helper.QueueStore,
		helper.ProcStore,
		helper.DAGRunMgr,
		helper.Config,
		nil,
		helper.ServiceRegistry,
		nil,
		nil,
		localapi.WithSchedulerStateStore(stubSchedulerStateStore{state: state}),
	)

	name := dag.Name
	listRespObj, err := api.ListDAGs(context.Background(), openapi.ListDAGsRequestObject{
		Params: openapi.ListDAGsParams{Name: &name},
	})
	require.NoError(t, err)

	listResp, ok := listRespObj.(*openapi.ListDAGs200JSONResponse)
	require.True(t, ok)
	require.Len(t, listResp.Dags, 1)
	require.NotNil(t, listResp.Dags[0].NextRun)
	require.True(t, scheduledAt.Equal(*listResp.Dags[0].NextRun))

	sseRespAny, err := api.GetDAGsListData(context.Background(), "name="+name)
	require.NoError(t, err)

	sseResp, ok := sseRespAny.(openapi.ListDAGs200JSONResponse)
	require.True(t, ok)
	require.Len(t, sseResp.Dags, 1)
	require.NotNil(t, sseResp.Dags[0].NextRun)
	require.True(t, listResp.Dags[0].NextRun.Equal(*sseResp.Dags[0].NextRun))
}

func TestGetDAGDetails_InvalidYAML_Returns200WithErrors(t *testing.T) {
	t.Parallel()

	helper := test.Setup(t, test.WithStatusPersistence())

	// Write an invalid YAML file directly to the DAGs directory
	invalidYAML := `this is not valid yaml: [unterminated`
	dagFile := helper.CreateDAGFile(t, helper.Config.Paths.DAGsDir, "invalid-dag", []byte(invalidYAML))
	fileName := filepath.Base(dagFile)
	// Strip .yaml extension to match how the API resolves filenames
	fileName = fileName[:len(fileName)-len(".yaml")]

	api := localapi.New(
		helper.DAGStore,
		helper.DAGRunStore,
		helper.QueueStore,
		helper.ProcStore,
		helper.DAGRunMgr,
		helper.Config,
		nil,
		helper.ServiceRegistry,
		nil,
		nil,
	)

	respObj, err := api.GetDAGDetails(context.Background(), openapi.GetDAGDetailsRequestObject{
		FileName: fileName,
	})
	// Should NOT return an error (which would become a 404/500)
	require.NoError(t, err)

	resp, ok := respObj.(openapi.GetDAGDetails200JSONResponse)
	require.True(t, ok, "expected 200 response, got %T", respObj)

	// Should contain build errors describing the YAML parse failure
	require.NotEmpty(t, resp.Errors, "expected build errors for invalid YAML")

	// File path should still be set
	require.NotNil(t, resp.FilePath)
	require.NotEmpty(t, *resp.FilePath)
}

func TestGetDAGDetails_NonExistent_Returns404(t *testing.T) {
	t.Parallel()

	helper := test.Setup(t, test.WithStatusPersistence())

	api := localapi.New(
		helper.DAGStore,
		helper.DAGRunStore,
		helper.QueueStore,
		helper.ProcStore,
		helper.DAGRunMgr,
		helper.Config,
		nil,
		helper.ServiceRegistry,
		nil,
		nil,
	)

	_, err := api.GetDAGDetails(context.Background(), openapi.GetDAGDetailsRequestObject{
		FileName: "does-not-exist",
	})
	// Should return an error (which becomes a 404)
	require.Error(t, err)
}

func TestGetDAGDetailsAndSpecIncludeNextRun(t *testing.T) {
	t.Parallel()

	helper := test.Setup(t, test.WithStatusPersistence())
	scheduledAt := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	dag := helper.DAG(t, fmt.Sprintf(`
name: dag-details-next-run
schedule:
  - at: "%s"
steps:
  - command: echo hi
`, scheduledAt.Format(time.RFC3339)))

	state := &scheduler.SchedulerState{
		Version: scheduler.SchedulerStateVersion,
		DAGs: map[string]scheduler.DAGWatermark{
			dag.Name: {
				OneOffs: map[string]scheduler.OneOffScheduleState{
					dag.Schedule[0].Fingerprint(): {
						ScheduledTime: scheduledAt,
						Status:        scheduler.OneOffStatusPending,
					},
				},
			},
		},
	}

	api := localapi.New(
		helper.DAGStore,
		helper.DAGRunStore,
		helper.QueueStore,
		helper.ProcStore,
		helper.DAGRunMgr,
		helper.Config,
		nil,
		helper.ServiceRegistry,
		nil,
		nil,
		localapi.WithSchedulerStateStore(stubSchedulerStateStore{state: state}),
	)

	detailsRespObj, err := api.GetDAGDetails(context.Background(), openapi.GetDAGDetailsRequestObject{
		FileName: dag.FileName(),
	})
	require.NoError(t, err)

	detailsResp, ok := detailsRespObj.(openapi.GetDAGDetails200JSONResponse)
	require.True(t, ok)
	require.NotNil(t, detailsResp.Dag)
	require.NotNil(t, detailsResp.Dag.NextRun)
	require.True(t, scheduledAt.Equal(*detailsResp.Dag.NextRun))

	specRespObj, err := api.GetDAGSpec(context.Background(), openapi.GetDAGSpecRequestObject{
		FileName: dag.FileName(),
	})
	require.NoError(t, err)

	specResp, ok := specRespObj.(*openapi.GetDAGSpec200JSONResponse)
	if !ok {
		valueResp, valueOK := specRespObj.(openapi.GetDAGSpec200JSONResponse)
		require.True(t, valueOK)
		specResp = &valueResp
	}
	require.NotNil(t, specResp.Dag)
	require.NotNil(t, specResp.Dag.NextRun)
	require.True(t, scheduledAt.Equal(*specResp.Dag.NextRun))
}
