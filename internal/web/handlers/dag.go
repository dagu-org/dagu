package handlers

import (
	"fmt"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/lo"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type dagStatus struct {
	Name string
	Vals []scheduler.NodeStatus
}

type Log struct {
	GridData []*dagStatus
	Logs     []*models.StatusFile
}

type dagResponse struct {
	Title      string
	Charset    string
	DAG        *controller.DAGStatus
	Tab        string
	Graph      string
	Definition string
	LogData    *Log
	LogUrl     string
	StepLog    *logFile
	ScLog      *logFile
	Errors     []string
}

type logFile struct {
	Step    *models.Node
	LogFile string
	Content string
}

const (
	dag_TabType_Status  = "status"
	dag_TabType_Spec    = "spec"
	dag_TabType_History = "history"
	dag_TabType_StepLog = "log"
	dag_TabType_ScLog   = "scheduler-log"
)

func newDAGResponse(dagName string, dag *controller.DAGStatus, tab string) *dagResponse {
	return &dagResponse{
		Title:      dagName,
		DAG:        dag,
		Tab:        tab,
		Definition: "",
		LogData:    nil,
		Errors:     []string{},
	}
}

func handleGetDAG() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "dag")
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		dn := dagNameFromCtx(r.Context())
		tab := tabNameFromCtx(r.Context())

		paramFile := getReqParam(r, "file")
		paramStep := getReqParam(r, "step")

		file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dn))
		dr := controller.NewDAGStatusReader(jsondb.New())
		d, err := dr.ReadStatus(file, false)
		if d == nil {
			encodeError(w, err)
			return
		}
		c := controller.New(d.DAG, jsondb.New())
		data := newDAGResponse(d.DAG.Name, d, tab)
		if err != nil {
			data.Errors = append(data.Errors, err.Error())
		}

		switch tab {
		case dag_TabType_Status:
		case dag_TabType_Spec:
			data.Definition, _ = dag.ReadFile(file)

		case dag_TabType_History:
			logs := controller.New(d.DAG, jsondb.New()).GetRecentStatuses(30)
			data.LogData = buildLog(logs)

		case dag_TabType_StepLog:
			if isJsonRequest(r) {
				data.StepLog, err = readStepLog(c, paramFile, paramStep, cfg.LogEncodingCharset)
				if err != nil {
					encodeError(w, err)
					return
				}
			}

		case dag_TabType_ScLog:
			if isJsonRequest(r) {
				data.ScLog, err = readSchedulerLog(c, paramFile)
				if err != nil {
					encodeError(w, err)
					return
				}
			}

		default:
		}

		if isJsonRequest(r) {
			renderJson(w, data)
		} else {
			renderFunc(w, data)
		}
	}
}

func isJsonRequest(r *http.Request) bool {
	return r.Header.Get("Accept") == "application/json"
}

func handlePostDAG() http.HandlerFunc {
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		value := r.FormValue("value")
		reqId := r.FormValue("request-id")
		step := r.FormValue("step")
		params := r.FormValue("params")

		dn := dagNameFromCtx(r.Context())

		file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dn))
		dr := controller.NewDAGStatusReader(jsondb.New())
		d, err := dr.ReadStatus(file, false)
		if err != nil && action != "save" {
			encodeError(w, err)
			return
		}
		c := controller.New(d.DAG, jsondb.New())

		switch action {
		case "start":
			if d.Status.Status == scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("already running: %w", errInvalidArgs))
				return
			}
			c.StartAsync(cfg.Command, cfg.WorkDir, params)

		case "suspend":
			sc := suspend.NewSuspendChecker(storage.NewStorage(config.Get().SuspendFlagsDir))
			_ = sc.ToggleSuspend(d.DAG, value == "true")

		case "stop":
			if d.Status.Status != scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("the DAG is not running: %w", errInvalidArgs))
				return
			}
			err = c.Stop()
			if err != nil {
				encodeError(w, fmt.Errorf("error trying to stop the DAG: %w", err))
				return
			}

		case "retry":
			if reqId == "" {
				encodeError(w, fmt.Errorf("request-id is required: %w", errInvalidArgs))
				return
			}
			err = c.Retry(cfg.Command, cfg.WorkDir, reqId)
			if err != nil {
				encodeError(w, err)
				return
			}

		case "mark-success":
			if d.Status.Status == scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
				return
			}
			if reqId == "" {
				encodeError(w, fmt.Errorf("request-id is required: %w", errInvalidArgs))
				return
			}
			if step == "" {
				encodeError(w, fmt.Errorf("step name is required: %w", errInvalidArgs))
				return
			}

			err = updateStatus(c, reqId, step, scheduler.NodeStatus_Success)
			if err != nil {
				encodeError(w, err)
				return
			}

		case "mark-failed":
			if d.Status.Status == scheduler.SchedulerStatus_Running {
				encodeError(w, fmt.Errorf("the DAG is still running: %w", errInvalidArgs))
				return
			}
			if reqId == "" {
				encodeError(w, fmt.Errorf("request-id is required: %w", errInvalidArgs))
				return
			}
			if step == "" {
				encodeError(w, fmt.Errorf("step name is required: %w", errInvalidArgs))
				return
			}

			err = updateStatus(c, reqId, step, scheduler.NodeStatus_Error)
			if err != nil {
				encodeError(w, err)
				return
			}

		case "save":
			err := c.UpdateDAGSpec(value)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return

		case "rename":
			newfile := nameWithExt(path.Join(cfg.DAGs, value))
			c := controller.New(d.DAG, jsondb.New())
			err := c.MoveDAG(file, newfile)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))

		default:
			encodeError(w, errInvalidArgs)
			return
		}

		http.Redirect(w, r, dn, http.StatusSeeOther)
	}
}

func handleDeleteDAG() http.HandlerFunc {
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		dn := dagNameFromCtx(r.Context())

		file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dn))
		dr := controller.NewDAGStatusReader(jsondb.New())
		d, err := dr.ReadStatus(file, false)
		if err != nil {
			encodeError(w, err)
		}

		c := controller.New(d.DAG, jsondb.New())

		err = c.DeleteDAG()

		if err != nil {
			encodeError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}

func updateStatus(c *controller.DAGController, reqId, step string, to scheduler.NodeStatus) error {
	status, err := c.GetStatusByRequestId(reqId)
	if err != nil {
		return err
	}
	found := false
	for i := range status.Nodes {
		if status.Nodes[i].Step.Name == step {
			status.Nodes[i].Status = to
			status.Nodes[i].StatusText = to.String()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("step was not found: %s", step)
	}
	return c.UpdateStatus(status)
}

func readSchedulerLog(c *controller.DAGController, file string) (*logFile, error) {
	f := ""
	if file == "" {
		s, err := c.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("error reading the last status")
		}
		f = s.Log
	} else {
		s, err := jsondb.ParseFile(file)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", file, err)
		}
		f = s.Log
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", f, err)
	}
	return &logFile{
		LogFile: f,
		Content: string(b),
	}, nil
}

func readStepLog(c *controller.DAGController, file, stepName, enc string) (*logFile, error) {
	var steps []*models.Node
	var stepm = map[string]*models.Node{
		constants.OnSuccess: nil,
		constants.OnFailure: nil,
		constants.OnCancel:  nil,
		constants.OnExit:    nil,
	}
	if file == "" {
		s, err := c.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("failed to read status")
		}
		steps = s.Nodes
		stepm[constants.OnSuccess] = s.OnSuccess
		stepm[constants.OnFailure] = s.OnFailure
		stepm[constants.OnCancel] = s.OnCancel
		stepm[constants.OnExit] = s.OnExit
	} else {
		s, err := jsondb.ParseFile(file)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", file, err)
		}
		steps = s.Nodes
		stepm[constants.OnSuccess] = s.OnSuccess
		stepm[constants.OnFailure] = s.OnFailure
		stepm[constants.OnCancel] = s.OnCancel
		stepm[constants.OnExit] = s.OnExit
	}
	var step *models.Node = nil
	for _, s := range steps {
		if s.Name == stepName {
			step = s
			break
		}
	}
	if v, ok := stepm[stepName]; ok {
		step = v
	}
	if step == nil {
		return nil, fmt.Errorf("step name was not found %s", stepName)
	}
	var b []byte
	var err error
	if strings.ToLower(enc) == "euc-jp" {
		b, err = readFile(step.Log, japanese.EUCJP.NewDecoder())
	} else {
		b, err = os.ReadFile(step.Log)
	}
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", step.Log, err)
	}
	return &logFile{
		LogFile: step.Log,
		Step:    step,
		Content: string(b),
	}, nil
}

func readFile(f string, decorder *encoding.Decoder) ([]byte, error) {
	r, err := os.Open(f)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", f, err)
	}
	defer func() {
		_ = r.Close()
	}()
	tr := transform.NewReader(r, decorder)
	ret, err := io.ReadAll(tr)
	return ret, err
}

func buildLog(logs []*models.StatusFile) *Log {
	tmp := map[string][]scheduler.NodeStatus{}
	add := func(step *models.Node, i int) {
		n := step.Name
		if _, ok := tmp[n]; !ok {
			tmp[n] = make([]scheduler.NodeStatus, len(logs))
		}
		tmp[n][i] = step.Status
	}
	for i, l := range logs {
		for _, s := range l.Status.Nodes {
			add(s, i)
		}
	}
	grid := []*dagStatus{}
	for k, v := range tmp {
		grid = append(grid, &dagStatus{Name: k, Vals: v})
	}
	sort.Slice(grid, func(i, c int) bool {
		return strings.Compare(grid[i].Name, grid[c].Name) <= 0
	})
	tmp = map[string][]scheduler.NodeStatus{}
	for i, l := range logs {
		if l.Status.OnSuccess != nil {
			add(l.Status.OnSuccess, i)
		}
		if l.Status.OnFailure != nil {
			add(l.Status.OnFailure, i)
		}
		if l.Status.OnCancel != nil {
			add(l.Status.OnCancel, i)
		}
		if l.Status.OnExit != nil {
			add(l.Status.OnExit, i)
		}
	}
	for _, h := range []string{constants.OnSuccess, constants.OnFailure, constants.OnCancel, constants.OnExit} {
		if v, ok := tmp[h]; ok {
			grid = append(grid, &dagStatus{Name: h, Vals: v})
		}
	}

	ret := &Log{
		Logs:     lo.Reverse(logs),
		GridData: grid,
	}

	return ret
}

func getReqParam(r *http.Request, key string) string {
	if ss, ok := r.URL.Query()[key]; ok {
		return ss[0]
	}
	return ""
}
