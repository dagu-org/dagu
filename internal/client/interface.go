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
	"path/filepath"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/model"
)

type Client interface {
	// CreateDAG creates a new DAG.
	CreateDAG(id string) (string, error)
	// GetDAGSpec returns the spec of the specific DAG.
	GetDAGSpec(id string) (string, error)
	// GetDAG returns the DAG object.
	GrepDAGs(pattern string) ([]*persistence.GrepResult, []string, error)
	// Rename renames the DAG.
	Rename(oldID, newID string) error
	// Stop stops the current DAG execution.
	Stop(dAG *dag.DAG) error
	// StartAsync starts the specific history asynchronously.
	StartAsync(dAG *dag.DAG, opts StartOptions)
	// Start starts the specific history.
	Start(dAG *dag.DAG, opts StartOptions) error
	// Restart restarts the specific history.
	Restart(dAG *dag.DAG, opts RestartOptions) error
	// Rerty retries the execution of the DAG.
	Retry(dAG *dag.DAG, requestID string) error
	// GetCurrentStatus returns the current status of the DAG.
	GetCurrentStatus(dAG *dag.DAG) (*model.Status, error)
	// GetLatestStatus returns the latest status of the DAG.
	GetLatestStatus(dAG *dag.DAG) (*model.Status, error)
	// GetLatestDAGStatus returns the latest status of the DAG.
	GetLatestDAGStatus(dagLocation string) (*DAGStatus, error)
	// GetStatusByRequestID returns the status of the specific history.
	GetStatusByRequestID(dAG *dag.DAG, requestID string) (*model.Status, error)
	// ListRecentHistory returns the recent history of the DAG.
	ListRecentHistory(dAG *dag.DAG, n int) []*model.History
	// UpdateStatus updates the status of the specific history.
	UpdateStatus(dAG *dag.DAG, status *model.Status) error
	// UpdateDAGSpec updates the spec of the specific DAG.
	UpdateDAGSpec(id string, spec string) error
	// DeleteDAG deletes the specific DAG.
	DeleteDAG(id, loc string) error
	// ListDAGStatusObsolete returns the list of statuses of the DAGs.
	ListDAGStatusObsolete() (statuses []*DAGStatus, errs []string, err error)
	// ListDAGStatus returns the list of statuses of the DAGs.
	ListDAGStatus(params dags.ListDagsParams) ([]*DAGStatus, *DagListPaginationSummaryResult, error)
	// ListHistoryByDate returns the history of the specific date.
	ListHistoryByDate(date string) ([]*model.History, error)
	// IsSuspended returns whether the DAG is suspended.
	IsSuspended(id string) bool
	// ToggleSuspend toggles the suspend status of the DAG.
	ToggleSuspend(id string, suspend bool) error
	// ListTags returns the list of tags of the DAGs.
	ListTags() ([]string, []string, error)
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
	Status    *model.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

type DagListPaginationSummaryResult struct {
	PageCount int
	ErrorList []string
}

func newDAGStatus(
	workflow *dag.DAG, s *model.Status, suspended bool, err error,
) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(workflow.Location),
		Dir:       filepath.Dir(workflow.Location),
		DAG:       workflow,
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
