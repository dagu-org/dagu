// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package client

import (
	"context"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/history"
)

type Client interface {
	// CreateDAG creates a new DAG.
	CreateDAG(ctx context.Context, id string) (string, error)
	// GetDAGSpec returns the spec of the specific DAG.
	GetDAGSpec(ctx context.Context, id string) (string, error)
	// GetDAG returns the DAG object.
	GrepDAGs(ctx context.Context, pattern string) ([]*persistence.GrepResult, []string, error)
	// Rename renames the DAG.
	Rename(ctx context.Context, oldID, newID string) error
	// Stop stops the current DAG execution.
	Stop(ctx context.Context, dAG *dag.DAG) error
	// StartAsync starts the specific history asynchronously.
	StartAsync(ctx context.Context, dAG *dag.DAG, opts StartOptions)
	// Start starts the specific history.
	Start(ctx context.Context, dAG *dag.DAG, opts StartOptions) error
	// Restart restarts the specific history.
	Restart(ctx context.Context, dAG *dag.DAG, opts RestartOptions) error
	// Rerty retries the execution of the DAG.
	Retry(ctx context.Context, dAG *dag.DAG, requestID string) error
	// GetCurrentStatus returns the current status of the DAG.
	GetCurrentStatus(ctx context.Context, dAG *dag.DAG) (*history.Status, error)
	// GetLatestStatus returns the latest status of the DAG.
	GetLatestStatus(ctx context.Context, dAG *dag.DAG) (*history.Status, error)
	// GetLatestDAGStatus returns the latest status of the DAG.
	GetLatestDAGStatus(ctx context.Context, dagLocation string) (*DAGStatus, error)
	// GetStatusByRequestID returns the status of the specific history.
	GetStatusByRequestID(ctx context.Context, dAG *dag.DAG, requestID string) (*history.Status, error)
	// ListRecentHistory returns the recent history of the DAG.
	ListRecentHistory(ctx context.Context, dAG *dag.DAG, n int) []*history.History
	// UpdateStatus updates the status of the specific history.
	UpdateStatus(ctx context.Context, dAG *dag.DAG, status *history.Status) error
	// UpdateDAGSpec updates the spec of the specific DAG.
	UpdateDAGSpec(ctx context.Context, id string, spec string) error
	// DeleteDAG deletes the specific DAG.
	DeleteDAG(ctx context.Context, id, loc string) error
	// ListDAGStatusObsolete returns the list of statuses of the DAGs.
	ListDAGStatusObsolete(ctx context.Context) (statuses []*DAGStatus, errs []string, err error)
	// ListDAGStatus returns the list of statuses of the DAGs.
	ListDAGStatus(ctx context.Context, params dags.ListDagsParams) ([]*DAGStatus, *DagListPaginationSummaryResult, error)
	// ListHistoryByDate returns the history of the specific date.
	ListHistoryByDate(ctx context.Context, date string) ([]*history.History, error)
	// IsSuspended returns whether the DAG is suspended.
	IsSuspended(ctx context.Context, id string) bool
	// ToggleSuspend toggles the suspend status of the DAG.
	ToggleSuspend(ctx context.Context, id string, suspend bool) error
	// ListTags returns the list of tags of the DAGs.
	ListTags(ctx context.Context) ([]string, []string, error)
}

type StartOptions struct {
	Params string
	Quiet  bool
}

type RestartOptions struct {
	Quiet bool
}

type DAGStatus struct {
	File      string
	Dir       string
	DAG       *dag.DAG
	Status    *history.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

type DagListPaginationSummaryResult struct {
	PageCount int
	ErrorList []string
}

func newDAGStatus(
	dAG *dag.DAG, s *history.Status, suspended bool, err error,
) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(dAG.Location),
		Dir:       filepath.Dir(dAG.Location),
		DAG:       dAG,
		Status:    s,
		Suspended: suspended,
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
