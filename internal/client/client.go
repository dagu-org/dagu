// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/sock"
)

// New creates a new Client instance.
// The Client is used to interact with the DAG.
func New(
	dataStore persistence.DataStores,
	executable string,
	workDir string,
	lg logger.Logger,
) Client {
	return &client{
		dataStore:  dataStore,
		executable: executable,
		workDir:    workDir,
		logger:     lg,
	}
}

type client struct {
	dataStore  persistence.DataStores
	executable string
	workDir    string
	logger     logger.Logger
}

var (
	dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

var (
	errCreateDAGFile = errors.New("failed to create DAG file")
	errGetStatus     = errors.New("failed to get status")
	errDAGIsRunning  = errors.New("the DAG is running")
)

func (e *client) GetDAGSpec(ctx context.Context, id string) (string, error) {
	dagStore := e.dataStore.DAGStore()
	return dagStore.GetSpec(ctx, id)
}

func (e *client) CreateDAG(ctx context.Context, name string) (string, error) {
	dagStore := e.dataStore.DAGStore()
	id, err := dagStore.Create(ctx, name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errCreateDAGFile, err)
	}
	return id, nil
}

func (e *client) Grep(ctx context.Context, pattern string) (
	[]*persistence.GrepResult, []string, error,
) {
	dagStore := e.dataStore.DAGStore()
	return dagStore.Grep(ctx, pattern)
}

func (e *client) Rename(ctx context.Context, oldID, newID string) error {
	dagStore := e.dataStore.DAGStore()
	oldDAG, err := dagStore.Find(ctx, oldID)
	if err != nil {
		return err
	}
	if err := dagStore.Rename(ctx, oldID, newID); err != nil {
		return err
	}
	newDAG, err := dagStore.Find(ctx, newID)
	if err != nil {
		return err
	}
	historyStore := e.dataStore.HistoryStore()
	return historyStore.Rename(ctx, oldDAG.Location, newDAG.Location)
}

func (e *client) Stop(ctx context.Context, workflow *digraph.DAG) error {
	// TODO: fix this not to connect to the DAG directly
	client := sock.NewClient(workflow.SockAddr())
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *client) StartAsync(ctx context.Context, workflow *digraph.DAG, opts StartOptions) {
	go func() {
		if err := e.Start(ctx, workflow, opts); err != nil {
			e.logger.Error("Workflow start operation failed", "error", err)
		}
	}()
}

func (e *client) Start(ctx context.Context, workflow *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(opts.Params)))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	args = append(args, workflow.Location)
	// nolint:gosec
	cmd := exec.Command(e.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = e.workDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (e *client) Restart(ctx context.Context, workflow *digraph.DAG, opts RestartOptions) error {
	args := []string{"restart"}
	if opts.Quiet {
		args = append(args, "-q")
	}
	args = append(args, workflow.Location)
	// nolint:gosec
	cmd := exec.Command(e.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = e.workDir
	cmd.Env = os.Environ()
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (e *client) Retry(ctx context.Context, workflow *digraph.DAG, requestID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--req=%s", requestID))
	args = append(args, workflow.Location)
	// nolint:gosec
	cmd := exec.Command(e.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = e.workDir
	cmd.Env = os.Environ()
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (*client) GetCurrentStatus(ctx context.Context, workflow *digraph.DAG) (*model.Status, error) {
	client := sock.NewClient(workflow.SockAddr())
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		}
		return model.NewStatusDefault(workflow), nil
	}
	return model.StatusFromJSON(ret)
}

func (e *client) GetStatusByRequestID(ctx context.Context, workflow *digraph.DAG, requestID string) (
	*model.Status, error,
) {
	ret, err := e.dataStore.HistoryStore().FindByRequestID(ctx, workflow.Location, requestID)
	if err != nil {
		return nil, err
	}
	status, _ := e.GetCurrentStatus(ctx, workflow)
	if status != nil && status.RequestID != requestID {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (*client) currentStatus(ctx context.Context, workflow *digraph.DAG) (*model.Status, error) {
	client := sock.NewClient(workflow.SockAddr())
	ret, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errGetStatus, err)
	}
	return model.StatusFromJSON(ret)
}

func (e *client) GetLatestStatus(ctx context.Context, workflow *digraph.DAG) (*model.Status, error) {
	currStatus, _ := e.currentStatus(ctx, workflow)
	if currStatus != nil {
		return currStatus, nil
	}
	status, err := e.dataStore.HistoryStore().ReadStatusToday(ctx, workflow.Location)
	if errors.Is(err, persistence.ErrNoStatusDataToday) ||
		errors.Is(err, persistence.ErrNoStatusData) {
		return model.NewStatusDefault(workflow), nil
	}
	if err != nil {
		return model.NewStatusDefault(workflow), err
	}
	status.CorrectRunningStatus()
	return status, nil
}

func (e *client) GetRecentHistory(ctx context.Context, workflow *digraph.DAG, n int) []*model.StatusFile {
	return e.dataStore.HistoryStore().ReadStatusRecent(ctx, workflow.Location, n)
}

func (e *client) UpdateStatus(ctx context.Context, workflow *digraph.DAG, status *model.Status) error {
	client := sock.NewClient(workflow.SockAddr())
	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		unmarshalled, _ := model.StatusFromJSON(res)
		if unmarshalled != nil && unmarshalled.RequestID == status.RequestID &&
			unmarshalled.Status == scheduler.StatusRunning {
			return errDAGIsRunning
		}
	}
	return e.dataStore.HistoryStore().Update(ctx, workflow.Location, status.RequestID, status)
}

func (e *client) UpdateDAG(ctx context.Context, id string, spec string) error {
	dagStore := e.dataStore.DAGStore()
	return dagStore.UpdateSpec(ctx, id, []byte(spec))
}

func (e *client) DeleteDAG(ctx context.Context, name, loc string) error {
	err := e.dataStore.HistoryStore().RemoveAll(ctx, loc)
	if err != nil {
		return err
	}
	dagStore := e.dataStore.DAGStore()
	return dagStore.Delete(ctx, name)
}

func (e *client) GetAllStatus(ctx context.Context) (
	statuses []*DAGStatus, errs []string, err error,
) {
	dagStore := e.dataStore.DAGStore()
	dagList, errs, err := dagStore.List(ctx)

	var ret []*DAGStatus
	for _, d := range dagList {
		status, err := e.readStatus(ctx, d)
		if err != nil {
			errs = append(errs, err.Error())
		}
		ret = append(ret, status)
	}

	return ret, errs, err
}

func (e *client) getPageCount(total int, limit int) int {
	return (total-1)/(limit) + 1
}

func (e *client) GetAllStatusPagination(ctx context.Context, params dags.ListDagsParams) ([]*DAGStatus, *DagListPaginationSummaryResult, error) {
	var (
		dagListPaginationResult *persistence.DagListPaginationResult
		err                     error
		dagStore                = e.dataStore.DAGStore()
		dagStatusList           = make([]*DAGStatus, 0)
		currentStatus           *DAGStatus
	)

	page := 1
	if params.Page != nil {
		page = int(*params.Page)
	}
	limit := 100
	if params.Limit != nil {
		limit = int(*params.Limit)
	}

	if dagListPaginationResult, err = dagStore.ListPagination(ctx, persistence.DAGListPaginationArgs{
		Page:  page,
		Limit: limit,
		Name:  params.SearchName,
		Tag:   params.SearchTag,
	}); err != nil {
		return dagStatusList, &DagListPaginationSummaryResult{PageCount: 1}, err
	}

	for _, currentDag := range dagListPaginationResult.DagList {
		if currentStatus, err = e.readStatus(ctx, currentDag); err != nil {
			dagListPaginationResult.ErrorList = append(dagListPaginationResult.ErrorList, err.Error())
		}
		dagStatusList = append(dagStatusList, currentStatus)
	}

	return dagStatusList, &DagListPaginationSummaryResult{
		PageCount: e.getPageCount(dagListPaginationResult.Count, limit),
		ErrorList: dagListPaginationResult.ErrorList,
	}, nil
}

func (e *client) getDAG(ctx context.Context, name string) (*digraph.DAG, error) {
	dagStore := e.dataStore.DAGStore()
	dagDetail, err := dagStore.GetDetails(ctx, name)
	return e.emptyDAGIfNil(dagDetail, name), err
}

func (e *client) GetStatus(ctx context.Context, id string) (*DAGStatus, error) {
	dag, err := e.getDAG(ctx, id)
	if dag == nil {
		// TODO: fix not to use location
		dag = &digraph.DAG{Name: id, Location: id}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(e.logger, dag.Steps...)
	}
	latestStatus, _ := e.GetLatestStatus(ctx, dag)
	return newDAGStatus(
		dag, latestStatus, e.IsSuspended(ctx, id), err,
	), err
}

func (e *client) ToggleSuspend(ctx context.Context, id string, suspend bool) error {
	flagStore := e.dataStore.FlagStore()
	return flagStore.ToggleSuspend(id, suspend)
}

func (e *client) readStatus(ctx context.Context, workflow *digraph.DAG) (*DAGStatus, error) {
	latestStatus, err := e.GetLatestStatus(ctx, workflow)
	id := strings.TrimSuffix(
		filepath.Base(workflow.Location),
		filepath.Ext(workflow.Location),
	)

	return newDAGStatus(
		workflow, latestStatus, e.IsSuspended(ctx, id), err,
	), err
}

func (*client) emptyDAGIfNil(workflow *digraph.DAG, dagLocation string) *digraph.DAG {
	if workflow != nil {
		return workflow
	}
	return &digraph.DAG{Location: dagLocation}
}

func (e *client) IsSuspended(ctx context.Context, id string) bool {
	flagStore := e.dataStore.FlagStore()
	return flagStore.IsSuspended(id)
}

func escapeArg(input string) string {
	escaped := strings.Builder{}

	for _, char := range input {
		if char == '\r' {
			_, _ = escaped.WriteString("\\r")
		} else if char == '\n' {
			_, _ = escaped.WriteString("\\n")
		} else {
			_, _ = escaped.WriteRune(char)
		}
	}

	return escaped.String()
}

func (e *client) GetTagList(ctx context.Context) ([]string, []string, error) {
	return e.dataStore.DAGStore().TagList(ctx)
}
