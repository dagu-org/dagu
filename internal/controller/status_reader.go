package controller

import (
	"fmt"
	"github.com/yohamta/dagu/internal/persistence"
	"os"
	"path/filepath"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
	"github.com/yohamta/dagu/internal/utils"
)

// DAGStatus is the struct to contain DAGStatus spec and status.
type DAGStatus struct {
	File      string
	Dir       string
	DAG       *dag.DAG
	Status    *models.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

// DAGStatusReader is the struct to read DAGStatus.
type DAGStatusReader struct {
	suspendChecker *suspend.SuspendChecker
	historyStore   persistence.HistoryStore
}

func NewDAGStatusReader(historyStore persistence.HistoryStore) *DAGStatusReader {
	return &DAGStatusReader{
		suspendChecker: suspend.NewSuspendChecker(
			storage.NewStorage(config.Get().SuspendFlagsDir),
		),
		historyStore: historyStore,
	}
}

// ReadAllStatus reads all DAGStatus
func (dr *DAGStatusReader) ReadAllStatus(DAGsDir string) (statuses []*DAGStatus, errs []string, err error) {
	statuses = []*DAGStatus{}
	errs = []string{}
	if !utils.FileExists(DAGsDir) {
		if err = os.MkdirAll(DAGsDir, 0755); err != nil {
			errs = append(errs, err.Error())
			return
		}
	}
	fis, err := os.ReadDir(DAGsDir)
	utils.LogErr("read DAGs directory", err)
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			d, err := dr.ReadStatus(filepath.Join(DAGsDir, fi.Name()), true)
			utils.LogErr("read DAG config", err)
			if d != nil {
				statuses = append(statuses, d)
			} else {
				errs = append(errs, fmt.Sprintf("reading %s failed: %s", fi.Name(), err))
			}
		}
	}
	return statuses, errs, nil
}

// ReadStatus loads DAG from config file.
func (dr *DAGStatusReader) ReadStatus(dagLocation string, loadMetadataOnly bool) (*DAGStatus, error) {
	var (
		cl  = dag.Loader{}
		d   *dag.DAG
		err error
	)

	if loadMetadataOnly {
		d, err = cl.LoadMetadataOnly(dagLocation)
	} else {
		d, err = cl.LoadWithoutEval(dagLocation)
	}

	if err != nil {
		if d != nil {
			return dr.newDAGStatus(d, defaultStatus(d), err), err
		}
		d := &dag.DAG{Location: dagLocation}
		return dr.newDAGStatus(d, defaultStatus(d), err), err
	}

	if !loadMetadataOnly {
		if _, err := scheduler.NewExecutionGraph(d.Steps...); err != nil {
			return dr.newDAGStatus(d, nil, err), err
		}
	}

	dc := New(d, dr.historyStore)
	status, err := dc.GetLastStatus()
	return dr.newDAGStatus(d, status, err), err
}

func (dr *DAGStatusReader) newDAGStatus(d *dag.DAG, s *models.Status, err error) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(d.Location),
		Dir:       filepath.Dir(d.Location),
		DAG:       d,
		Status:    s,
		Suspended: dr.suspendChecker.IsSuspended(d),
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
