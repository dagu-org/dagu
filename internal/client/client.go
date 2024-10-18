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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
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

func (e *client) GetDAGSpec(id string) (string, error) {
	dagStore := e.dataStore.DAGStore()
	return dagStore.GetSpec(id)
}

func (e *client) CreateDAG(name string) (string, error) {
	dagStore := e.dataStore.DAGStore()
	id, err := dagStore.Create(name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errCreateDAGFile, err)
	}
	return id, nil
}

func (e *client) Grep(pattern string) (
	[]*persistence.GrepResult, []string, error,
) {
	dagStore := e.dataStore.DAGStore()
	return dagStore.Grep(pattern)
}

func (e *client) Rename(oldID, newID string) error {
	dagStore := e.dataStore.DAGStore()
	oldDAG, err := dagStore.Find(oldID)
	if err != nil {
		return err
	}
	if err := dagStore.Rename(oldID, newID); err != nil {
		return err
	}
	newDAG, err := dagStore.Find(newID)
	if err != nil {
		return err
	}
	historyStore := e.dataStore.HistoryStore()
	return historyStore.Rename(oldDAG.Location, newDAG.Location)
}

func (e *client) Stop(workflow *dag.DAG) error {
	// TODO: fix this not to connect to the DAG directly
	client := sock.NewClient(workflow.SockAddr())
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *client) StartAsync(workflow *dag.DAG, opts StartOptions) {
	go func() {
		if err := e.Start(workflow, opts); err != nil {
			e.logger.Error("Workflow start operation failed", "error", err)
		}
	}()
}

func (e *client) Start(workflow *dag.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(opts.Params)))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.FromWaitingQueue {
		args = append(args, "-w")
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

func (e *client) StartFromQueue(workflow *dag.DAG, opts StartOptions) error {
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

func (e *client) Restart(workflow *dag.DAG, opts RestartOptions) error {
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

func (e *client) Retry(workflow *dag.DAG, requestID string) error {
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

func (*client) GetCurrentStatus(workflow *dag.DAG) (*model.Status, error) {
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

func (e *client) GetStatusByRequestID(workflow *dag.DAG, requestID string) (
	*model.Status, error,
) {
	ret, err := e.dataStore.HistoryStore().FindByRequestID(
		workflow.Location, requestID,
	)
	if err != nil {
		return nil, err
	}
	status, _ := e.GetCurrentStatus(workflow)
	if status != nil && status.RequestID != requestID {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (*client) currentStatus(workflow *dag.DAG) (*model.Status, error) {
	client := sock.NewClient(workflow.SockAddr())
	ret, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errGetStatus, err)
	}
	return model.StatusFromJSON(ret)
}

func (e *client) GetLatestStatus(workflow *dag.DAG) (*model.Status, error) {
	currStatus, _ := e.currentStatus(workflow)
	if currStatus != nil {
		return currStatus, nil
	}
	status, err := e.dataStore.HistoryStore().ReadStatusToday(workflow.Location)
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

func (e *client) GetRecentHistory(workflow *dag.DAG, n int) []*model.StatusFile {
	return e.dataStore.HistoryStore().ReadStatusRecent(workflow.Location, n)
}

func (e *client) UpdateStatus(workflow *dag.DAG, status *model.Status) error {
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
	return e.dataStore.HistoryStore().Update(
		workflow.Location, status.RequestID, status,
	)
}

func (e *client) UpdateDAG(id string, spec string) error {
	dagStore := e.dataStore.DAGStore()
	return dagStore.UpdateSpec(id, []byte(spec))
}

func (e *client) DeleteDAG(name, loc string) error {
	err := e.dataStore.HistoryStore().RemoveAll(loc)
	if err != nil {
		return err
	}
	dagStore := e.dataStore.DAGStore()
	return dagStore.Delete(name)
}

func (e *client) GetAllStatus() (
	statuses []*DAGStatus, errs []string, err error,
) {
	dagStore := e.dataStore.DAGStore()
	dagList, errs, err := dagStore.List()

	var ret []*DAGStatus
	for _, d := range dagList {
		status, err := e.readStatus(d)
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

func (e *client) GetAllStatusPagination(params dags.ListDagsParams) ([]*DAGStatus, *DagListPaginationSummaryResult, error) {
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

	if dagListPaginationResult, err = dagStore.ListPagination(persistence.DAGListPaginationArgs{
		Page:  page,
		Limit: limit,
		Name:  params.SearchName,
		Tag:   params.SearchTag,
	}); err != nil {
		return dagStatusList, &DagListPaginationSummaryResult{PageCount: 1}, err
	}

	for _, currentDag := range dagListPaginationResult.DagList {
		if currentStatus, err = e.readStatus(currentDag); err != nil {
			dagListPaginationResult.ErrorList = append(dagListPaginationResult.ErrorList, err.Error())
		}
		dagStatusList = append(dagStatusList, currentStatus)
	}

	return dagStatusList, &DagListPaginationSummaryResult{
		PageCount: e.getPageCount(dagListPaginationResult.Count, limit),
		ErrorList: dagListPaginationResult.ErrorList,
	}, nil
}

func (e *client) getDAG(name string) (*dag.DAG, error) {
	dagStore := e.dataStore.DAGStore()
	dagDetail, err := dagStore.GetDetails(name)
	return e.emptyDAGIfNil(dagDetail, name), err
}

func (e *client) GetStatus(id string) (*DAGStatus, error) {
	dg, err := e.getDAG(id)
	if dg == nil {
		// TODO: fix not to use location
		dg = &dag.DAG{Name: id, Location: id}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(e.logger, dg.Steps...)
	}
	latestStatus, _ := e.GetLatestStatus(dg)
	return newDAGStatus(
		dg, latestStatus, e.IsSuspended(id), err,
	), err
}

func (e *client) ToggleSuspend(id string, suspend bool) error {
	flagStore := e.dataStore.FlagStore()
	return flagStore.ToggleSuspend(id, suspend)
}

func (e *client) readStatus(workflow *dag.DAG) (*DAGStatus, error) {
	latestStatus, err := e.GetLatestStatus(workflow)
	id := strings.TrimSuffix(
		filepath.Base(workflow.Location),
		filepath.Ext(workflow.Location),
	)

	return newDAGStatus(
		workflow, latestStatus, e.IsSuspended(id), err,
	), err
}

func (*client) emptyDAGIfNil(workflow *dag.DAG, dagLocation string) *dag.DAG {
	if workflow != nil {
		return workflow
	}
	return &dag.DAG{Location: dagLocation}
}

func (e *client) IsSuspended(id string) bool {
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

func (e *client) GetTagList() ([]string, []string, error) {
	return e.dataStore.DAGStore().TagList()
}
