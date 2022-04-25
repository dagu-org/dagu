package handlers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yohamta/jobctl/internal/config"
	"github.com/yohamta/jobctl/internal/constants"
	"github.com/yohamta/jobctl/internal/controller"
	"github.com/yohamta/jobctl/internal/database"
	"github.com/yohamta/jobctl/internal/models"
	"github.com/yohamta/jobctl/internal/scheduler"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type jobStatus struct {
	Name string
	Vals []scheduler.NodeStatus
}

type Log struct {
	GridData []*jobStatus
	Logs     []*models.StatusFile
}

type jobResponse struct {
	Title      string
	Charset    string
	Job        *controller.Job
	Tab        jobTabType
	Graph      string
	Definition string
	LogData    *Log
	LogUrl     string
	Group      string
	StepLog    *stepLog
	ScLog      *schedulerLog
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

type jobTabType int

const (
	JobTabType_Status jobTabType = iota
	JobTabType_Config
	JobTabType_History
	JobTabType_StepLog
	JobTabType_ScLog
	JobTabType_None
)

type jobParameter struct {
	Tab   jobTabType
	Group string
	File  string
	Step  string
}

func newJobResponse(cfg string, job *controller.Job, tab jobTabType,
	group string) *jobResponse {
	return &jobResponse{
		Title:      cfg,
		Job:        job,
		Tab:        tab,
		Definition: "",
		LogData:    nil,
		Group:      group,
	}
}

type JobHandlerConfig struct {
	JobsDir            string
	LogEncodingCharset string
}

func HandleGetJob(hc *JobHandlerConfig) http.HandlerFunc {
	renderFunc := useTemplate("job.gohtml", "job")

	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := getPathParameter(r)
		if err != nil {
			encodeError(w, err)
			return
		}

		params := getJobParameter(r)
		job, err := controller.FromConfig(filepath.Join(hc.JobsDir, params.Group, cfg))
		if err != nil {
			encodeError(w, err)
			return
		}
		c := controller.New(job.Config)
		data := newJobResponse(cfg, job, params.Tab, params.Group)

		switch params.Tab {
		case JobTabType_Status:
			data.Graph = models.StepGraph(job.Status.Nodes, params.Tab != JobTabType_Config)
		case JobTabType_Config:
			steps := models.FromSteps(job.Config.Steps)
			data.Graph = models.StepGraph(steps, params.Tab != JobTabType_Config)
			data.Definition, _ = config.ReadConfig(path.Join(hc.JobsDir, params.Group, cfg))
		case JobTabType_History:
			logs, err := controller.New(job.Config).GetStatusHist(30)
			if err != nil {
				encodeError(w, err)
				return
			}
			data.LogData = buildLog(logs)
		case JobTabType_StepLog:
			if isJsonRequest(r) {
				data.StepLog, err = readStepLog(c, params.File, params.Step, hc.LogEncodingCharset)
				if err != nil {
					encodeError(w, err)
					return
				}
			}
		case JobTabType_ScLog:
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

type PostJobHandlerConfig struct {
	JobsDir string
	Bin     string
	WkDir   string
}

func HandlePostJobAction(hc *PostJobHandlerConfig) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		group := r.FormValue("group")
		reqId := r.FormValue("request-id")

		cfg, err := getPathParameter(r)
		if err != nil {
			encodeError(w, err)
			return
		}

		file := filepath.Join(hc.JobsDir, group, cfg)
		job, err := controller.FromConfig(file)
		if err != nil {
			encodeError(w, err)
			return
		}
		c := controller.New(job.Config)

		switch action {
		case "start":
			if job.Status.Status == scheduler.SchedulerStatus_Running {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("job is already running."))
				return
			}
			err = c.StartJob(hc.Bin, hc.WkDir, "")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		case "stop":
			if job.Status.Status != scheduler.SchedulerStatus_Running {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("job is not running."))
				return
			}
			err = c.StopJob()
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
			err = c.RetryJob(hc.Bin, hc.WkDir, reqId)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		default:
			encodeError(w, errInvalidArgs)
			return
		}

		http.Redirect(w, r, job.File, http.StatusSeeOther)
	}
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
		logFile = s.Status.Log
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
		steps = s.Status.Nodes
		stepm[constants.OnSuccess] = s.Status.OnSuccess
		stepm[constants.OnFailure] = s.Status.OnFailure
		stepm[constants.OnCancel] = s.Status.OnCancel
		stepm[constants.OnExit] = s.Status.OnExit
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
	ret, err := ioutil.ReadAll(tr)
	return ret, err
}

func buildLog(logs []*models.StatusFile) *Log {
	ret := &Log{
		GridData: []*jobStatus{},
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
		ret.GridData = append(ret.GridData, &jobStatus{Name: k, Vals: v})
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
			ret.GridData = append(ret.GridData, &jobStatus{Name: h, Vals: v})
		}
	}
	return ret
}

func getPathParameter(r *http.Request) (string, error) {
	re := regexp.MustCompile("/jobs/([^/\\?]+)/?$")
	m := re.FindStringSubmatch(r.URL.Path)
	if len(m) < 2 {
		return "", fmt.Errorf("invalid URL")
	}
	return m[1], nil
}

func getJobParameter(r *http.Request) *jobParameter {
	p := &jobParameter{
		Tab:   JobTabType_Status,
		Group: "",
	}
	if tab, ok := r.URL.Query()["t"]; ok {
		i, err := strconv.Atoi(tab[0])
		if err != nil || i >= int(JobTabType_None) {
			p.Tab = JobTabType_Status
		} else {
			p.Tab = jobTabType(i)
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
