package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
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
	DAG        *controller.DAG
	Tab        dagTabType
	Graph      string
	Definition string
	LogData    *Log
	LogUrl     string
	Group      string
	StepLog    *stepLog
	ScLog      *schedulerLog
	Errors     []string
}

type schedulerLog struct {
	LogFile string
	Content string
}

type stepLog struct {
	Step    *models.Node
	LogFile string
	Content string
}

type dagTabType int

const (
	DAG_TabType_Status dagTabType = iota
	DAG_TabType_Config
	DAG_TabType_History
	DAG_TabType_StepLog
	DAG_TabType_ScLog
	DAG_TabType_None
)

type dagParameter struct {
	Tab   dagTabType
	Group string
	File  string
	Step  string
}

func newDAGResponse(cfg string, dag *controller.DAG, tab dagTabType,
	group string) *dagResponse {
	return &dagResponse{
		Title:      cfg,
		DAG:        dag,
		Tab:        tab,
		Definition: "",
		LogData:    nil,
		Group:      group,
		Errors:     []string{},
	}
}

type DAGHandlerConfig struct {
	DAGsDir            string
	LogEncodingCharset string
}

func HandleGetDAG(hc *DAGHandlerConfig) http.HandlerFunc {
	renderFunc := useTemplate("dag.gohtml", "dag")

	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := getPathParameter(r)
		if err != nil {
			encodeError(w, err)
			return
		}

		params := getDAGParameter(r)
		dag, err := controller.FromConfig(filepath.Join(hc.DAGsDir, params.Group, cfg))
		if dag == nil {
			encodeError(w, err)
			return
		}
		c := controller.New(dag.Config)
		data := newDAGResponse(cfg, dag, params.Tab, params.Group)
		if err != nil {
			data.Errors = append(data.Errors, err.Error())
		}

		switch params.Tab {
		case DAG_TabType_Status:
		case DAG_TabType_Config:
			data.Definition, _ = config.ReadConfig(path.Join(hc.DAGsDir, params.Group, cfg))

		case DAG_TabType_History:
			logs := controller.New(dag.Config).GetStatusHist(30)
			data.LogData = buildLog(logs)

		case DAG_TabType_StepLog:
			if isJsonRequest(r) {
				data.StepLog, err = readStepLog(c, params.File, params.Step, hc.LogEncodingCharset)
				if err != nil {
					encodeError(w, err)
					return
				}
			}

		case DAG_TabType_ScLog:
			if isJsonRequest(r) {
				data.ScLog, err = readSchedulerLog(c, params.File)
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

type PostDAGHandlerConfig struct {
	DAGsDir string
	Bin     string
	WkDir   string
}

func HandlePostDAGAction(hc *PostDAGHandlerConfig) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		value := r.FormValue("value")
		group := r.FormValue("group")
		reqId := r.FormValue("request-id")
		step := r.FormValue("step")

		cfg, err := getPathParameter(r)
		if err != nil {
			encodeError(w, err)
			return
		}

		file := filepath.Join(hc.DAGsDir, group, cfg)
		dag, err := controller.FromConfig(file)
		if err != nil && action != "save" {
			encodeError(w, err)
			return
		}
		c := controller.New(dag.Config)

		switch action {
		case "start":
			if dag.Status.Status == scheduler.SchedulerStatus_Running {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("DAG is already running."))
				return
			}
			err = c.Start(hc.Bin, hc.WkDir, "")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

		case "stop":
			if dag.Status.Status != scheduler.SchedulerStatus_Running {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("DAG is not running."))
				return
			}
			err = c.Stop()
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(err.Error()))
				return
			}

		case "retry":
			if reqId == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("request-id is required."))
				return
			}
			err = c.Retry(hc.Bin, hc.WkDir, reqId)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

		case "mark-success":
			if dag.Status.Status == scheduler.SchedulerStatus_Running {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("DAG is running."))
				return
			}
			if reqId == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("request-id is required."))
				return
			}
			if step == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("step is required."))
				return
			}

			err = updateStatus(c, reqId, step, scheduler.NodeStatus_Success)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

		case "mark-failed":
			if dag.Status.Status == scheduler.SchedulerStatus_Running {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("DAG is running."))
				return
			}
			if reqId == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("request-id is required."))
				return
			}
			if step == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("step is required."))
				return
			}

			err = updateStatus(c, reqId, step, scheduler.NodeStatus_Error)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

		case "save":
			err := c.Save(value)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		case "rename":
			newFile := path.Join(hc.DAGsDir, group, value)
			err := controller.RenameConfig(file, newFile)
			if err != nil {
				encodeError(w, err)
				return
			}
			c, _ := controller.FromConfig(newFile)
			group := strings.TrimLeft(strings.Replace(c.Dir, hc.DAGsDir, "", 1), "/")
			url := fmt.Sprintf("%s?group=%s&t=%d", path.Base(newFile), group, DAG_TabType_Config)
			http.Redirect(w, r, url, http.StatusSeeOther)
		default:
			encodeError(w, errInvalidArgs)
			return
		}

		http.Redirect(w, r, dag.File, http.StatusSeeOther)
	}
}

func updateStatus(c controller.Controller, reqId, step string, to scheduler.NodeStatus) error {
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
		return fmt.Errorf("step %s not found", step)
	}
	return c.UpdateStatus(status)
}

func readSchedulerLog(c controller.Controller, file string) (*schedulerLog, error) {
	logFile := ""
	if file == "" {
		s, err := c.GetLastStatus()
		if err != nil {
			return nil, fmt.Errorf("failed to read status")
		}
		logFile = s.Log
	} else {
		s, err := database.ParseFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read status file %s", file)
		}
		logFile = s.Log
	}
	b, err := os.ReadFile(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s", logFile)
	}
	return &schedulerLog{
		LogFile: file,
		Content: string(b),
	}, nil
}

func readStepLog(c controller.Controller, file, stepName, enc string) (*stepLog, error) {
	var steps []*models.Node = nil
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
		s, err := database.ParseFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read status file %s", file)
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
		return nil, fmt.Errorf("step was not found %s", stepName)
	}
	var b []byte = nil
	var err error = nil
	if strings.ToLower(enc) == "euc-jp" {
		b, err = readFile(step.Log, japanese.EUCJP.NewDecoder())
	} else {
		b, err = os.ReadFile(step.Log)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s", step.Log)
	}
	return &stepLog{
		LogFile: file,
		Step:    step,
		Content: string(b),
	}, nil
}

func readFile(f string, decorder *encoding.Decoder) ([]byte, error) {
	r, err := os.Open(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s", f)
	}
	defer r.Close()
	tr := transform.NewReader(r, decorder)
	ret, err := io.ReadAll(tr)
	return ret, err
}

func buildLog(logs []*models.StatusFile) *Log {
	ret := &Log{
		GridData: []*dagStatus{},
		Logs:     logs,
	}
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
	for k, v := range tmp {
		ret.GridData = append(ret.GridData, &dagStatus{Name: k, Vals: v})
	}
	sort.Slice(ret.GridData, func(i, c int) bool {
		return strings.Compare(ret.GridData[i].Name, ret.GridData[c].Name) <= 0
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
			ret.GridData = append(ret.GridData, &dagStatus{Name: h, Vals: v})
		}
	}
	return ret
}

func getPathParameter(r *http.Request) (string, error) {
	re := regexp.MustCompile(`/dags/([^/\?]+)/?$`)
	m := re.FindStringSubmatch(r.URL.Path)
	if len(m) < 2 {
		return "", fmt.Errorf("invalid URL")
	}
	return m[1], nil
}

func getDAGParameter(r *http.Request) *dagParameter {
	p := &dagParameter{
		Tab:   DAG_TabType_Status,
		Group: "",
	}
	if tab, ok := r.URL.Query()["t"]; ok {
		i, err := strconv.Atoi(tab[0])
		if err != nil || i >= int(DAG_TabType_None) {
			p.Tab = DAG_TabType_Status
		} else {
			p.Tab = dagTabType(i)
		}
	}
	if group, ok := r.URL.Query()["group"]; ok {
		p.Group = group[0]
	}
	if file, ok := r.URL.Query()["file"]; ok {
		p.File = file[0]
	}
	if step, ok := r.URL.Query()["step"]; ok {
		p.Step = step[0]
	}
	return p
}
