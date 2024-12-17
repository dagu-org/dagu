// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"path/filepath"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/model"
)

type Client interface {
	CreateDAG(id string) (string, error)
	GetDAGSpec(id string) (string, error)
	Grep(pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(oldID, newID string) error
	Stop(workflow *digraph.DAG) error
	StartAsync(workflow *digraph.DAG, opts StartOptions)
	Start(workflow *digraph.DAG, opts StartOptions) error
	Restart(workflow *digraph.DAG, opts RestartOptions) error
	Retry(workflow *digraph.DAG, requestID string) error
	GetCurrentStatus(workflow *digraph.DAG) (*model.Status, error)
	GetStatusByRequestID(workflow *digraph.DAG, requestID string) (*model.Status, error)
	GetLatestStatus(workflow *digraph.DAG) (*model.Status, error)
	GetRecentHistory(workflow *digraph.DAG, n int) []*model.StatusFile
	UpdateStatus(workflow *digraph.DAG, status *model.Status) error
	UpdateDAG(id string, spec string) error
	DeleteDAG(id, loc string) error
	GetAllStatus() (statuses []*DAGStatus, errs []string, err error)
	GetAllStatusPagination(params dags.ListDagsParams) ([]*DAGStatus, *DagListPaginationSummaryResult, error)
	GetStatus(dagLocation string) (*DAGStatus, error)
	IsSuspended(id string) bool
	ToggleSuspend(id string, suspend bool) error
	GetTagList() ([]string, []string, error)
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
	DAG       *digraph.DAG
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
	workflow *digraph.DAG, s *model.Status, suspended bool, err error,
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
