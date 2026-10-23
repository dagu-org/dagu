// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/auth"
	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/persis"
	"github.com/dagucloud/dagu/v2/internal/testutil"
	"github.com/stretchr/testify/require"
)

// The outputs helper resolves root and child attempts through the same access
// checks as the native handler: a caller visible to the child but not the
// root must not reach it.
func TestGetDAGRunOutputsDataReadsRootAndSubRun(t *testing.T) {
	ctx := t.Context()
	repository := testutil.NewFileDAGRunRepository(t.TempDir(), persis.DAGRunRepositoryOptions{})

	report := `{"ok":true,"rows":3}`
	rootDAG := &ir.DAG{Name: "root", Labels: ir.NewLabels([]string{"workspace=ops"})}
	rootRef := ir.NewDAGRunRef(rootDAG.Name, "root-run")
	rootAttempt, err := repository.CreateAttempt(ctx, rootDAG, time.Now(), rootRef.ID, persis.DAGRunCreateAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, rootAttempt.Open(ctx))
	rootStatus := ir.InitialStatus(rootDAG)
	rootStatus.DAGRunID = rootRef.ID
	rootStatus.AttemptID = rootAttempt.ID()
	require.NoError(t, rootAttempt.Write(ctx, rootStatus))
	require.NoError(t, rootAttempt.WriteOutputs(ctx, &ir.DAGRunOutputs{
		Metadata: ir.OutputsMetadata{DAGName: rootDAG.Name, DAGRunID: rootRef.ID, AttemptID: rootAttempt.ID(), Status: "succeeded"},
		Outputs:  map[string]string{"report": report},
	}))
	require.NoError(t, rootAttempt.Close(ctx))

	childDAG := &ir.DAG{Name: "child", Labels: ir.NewLabels([]string{"workspace=secret"})}
	childAttempt, err := repository.CreateAttempt(ctx, childDAG, time.Now(), "child-run", persis.DAGRunCreateAttemptOptions{
		RootDAGRun: rootRef,
	})
	require.NoError(t, err)
	require.NoError(t, childAttempt.Open(ctx))
	childStatus := ir.InitialStatus(childDAG)
	childStatus.Root = rootRef
	childStatus.DAGRunID = "child-run"
	childStatus.AttemptID = childAttempt.ID()
	require.NoError(t, childAttempt.Write(ctx, childStatus))
	require.NoError(t, childAttempt.WriteOutputs(ctx, &ir.DAGRunOutputs{
		Metadata: ir.OutputsMetadata{DAGName: childDAG.Name, DAGRunID: "child-run", AttemptID: childAttempt.ID(), Status: "succeeded"},
		Outputs:  map[string]string{"child_report": report},
	}))
	require.NoError(t, childAttempt.Close(ctx))

	cfg := &config.Config{}
	a := &API{
		authService:      struct{ AuthService }{},
		dagRunRepository: repository,
		config:           cfg,
	}
	fullAccess := auth.WithUser(ctx, &auth.User{
		Role: auth.RoleViewer,
		WorkspaceAccess: &auth.WorkspaceAccess{Grants: []auth.WorkspaceGrant{
			{Workspace: "ops", Role: auth.RoleViewer},
			{Workspace: "secret", Role: auth.RoleViewer},
		}},
	})

	outputs, err := a.GetDAGRunOutputsData(fullAccess, rootRef, "")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"report": report}, outputs.Outputs)

	// latest resolves to the same run.
	latest, err := a.GetDAGRunOutputsData(fullAccess, ir.NewDAGRunRef(rootDAG.Name, "latest"), "")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"report": report}, latest.Outputs)

	child, err := a.GetDAGRunOutputsData(fullAccess, rootRef, "child-run")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"child_report": report}, child.Outputs)

	secretOnly := auth.WithUser(t.Context(), &auth.User{
		Role: auth.RoleViewer,
		WorkspaceAccess: &auth.WorkspaceAccess{Grants: []auth.WorkspaceGrant{
			{Workspace: "secret", Role: auth.RoleViewer},
		}},
	})

	_, err = a.GetDAGRunOutputsData(secretOnly, rootRef, "")
	var apiErr *Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "Resource not found", apiErr.Message)

	// The child workspace is visible but the root is not, so the child stays
	// unreachable through the root address.
	_, err = a.GetDAGRunOutputsData(secretOnly, rootRef, "child-run")
	apiErr = nil
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "Resource not found", apiErr.Message)
}
