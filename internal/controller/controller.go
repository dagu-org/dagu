package controller

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/sock"
	"github.com/yohamta/dagu/internal/utils"
	"github.com/yohamta/grep"
)

// GetDAGs returns all DAGs in the config file.
func GetDAGs(dir string) (dags []*DAGStatus, errs []string, err error) {
	dags = []*DAGStatus{}
	errs = []string{}
	if !utils.FileExists(dir) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			errs = append(errs, err.Error())
			return
		}
	}
	fis, err := os.ReadDir(dir)
	utils.LogErr("read DAGs directory", err)
	dr := NewDAGReader()
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			dag, err := dr.ReadDAG(filepath.Join(dir, fi.Name()), true)
			utils.LogErr("read DAG config", err)
			if dag != nil {
				dags = append(dags, dag)
			} else {
				errs = append(errs, fmt.Sprintf("reading %s failed: %s", fi.Name(), err))
			}
		}
	}
	return dags, errs, nil
}

type GrepResult struct {
	Name    string
	DAG     *dag.DAG
	Matches []*grep.Match
}

// GrepDAGs returns all DAGs that contain the given string.
func GrepDAGs(dir string, pattern string) (ret []*GrepResult, errs []string, err error) {
	ret = []*GrepResult{}
	errs = []string{}
	if !utils.FileExists(dir) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			errs = append(errs, err.Error())
			return
		}
	}
	fis, err := os.ReadDir(dir)
	dl := &dag.Loader{}
	opts := &grep.Options{
		IsRegexp: true,
		Before:   2,
		After:    2,
	}
	utils.LogErr("read DAGs directory", err)
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			fn := filepath.Join(dir, fi.Name())
			utils.LogErr("read DAG file", err)
			m, err := grep.Grep(fn, fmt.Sprintf("(?i)%s", pattern), opts)
			if err != nil {
				continue
			} else if err != nil {
				errs = append(errs, fmt.Sprintf("grep %s failed: %s", fi.Name(), err))
				continue
			}
			dag, err := dl.LoadHeadOnly(fn)
			if err != nil {
				errs = append(errs, fmt.Sprintf("check %s failed: %s", fi.Name(), err))
				continue
			}
			ret = append(ret, &GrepResult{
				Name:    strings.TrimSuffix(fi.Name(), path.Ext(fi.Name())),
				DAG:     dag,
				Matches: m,
			})
		}
	}
	return ret, errs, nil
}

// NewConfig returns a new config.Config.
func NewConfig(file string) error {
	if err := assertPath(file); err != nil {
		return err
	}
	if utils.FileExists(file) {
		return fmt.Errorf("the config file %s already exists", file)
	}
	defaultVal := `steps:
  - name: step1
    command: echo hello
`
	return os.WriteFile(file, []byte(defaultVal), 0644)
}

// RenameConfig renames the config file and status database.
func RenameConfig(oldPath, newPath string) error {
	if err := assertPath(newPath); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	return defaultDb().MoveData(oldPath, newPath)
}

type Controller struct {
	*dag.DAG
}

func New(d *dag.DAG) *Controller {
	return &Controller{
		DAG: d,
	}
}

func (c *Controller) Stop() error {
	client := sock.Client{Addr: c.SockAddr()}
	_, err := client.Request("POST", "/stop")
	return err
}

func (c *Controller) StartAsync(bin string, workDir string, params string) {
	go func() {
		err := c.Start(bin, workDir, params)
		utils.LogErr("starting a DAG", err)
	}()
}

func (c *Controller) Start(bin string, workDir string, params string) error {
	args := []string{"start"}
	if params != "" {
		args = append(args, fmt.Sprintf("--params=\"%s\"", params))
	}
	args = append(args, c.Location)
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func (c *Controller) Retry(bin string, workDir string, reqId string) (err error) {
	go func() {
		args := []string{"retry"}
		args = append(args, fmt.Sprintf("--req=%s", reqId))
		args = append(args, c.Location)
		cmd := exec.Command(bin, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
		cmd.Dir = workDir
		cmd.Env = os.Environ()
		defer cmd.Wait()
		err = cmd.Start()
		utils.LogErr("retry a DAG", err)
	}()
	time.Sleep(time.Millisecond * 500)
	return
}

func (c *Controller) GetStatus() (*models.Status, error) {
	client := sock.Client{Addr: c.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		} else {
			return defaultStatus(c.DAG), nil
		}
	}
	return models.StatusFromJson(ret)
}

func (c *Controller) GetLastStatus() (*models.Status, error) {
	client := sock.Client{Addr: c.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err == nil {
		return models.StatusFromJson(ret)
	}
	if err == nil || !errors.Is(err, sock.ErrTimeout) {
		status, err := defaultDb().ReadStatusToday(c.Location)
		if err != nil {
			var readErr error = nil
			if err != database.ErrNoStatusDataToday && err != database.ErrNoStatusData {
				fmt.Printf("read status failed : %s", err)
				readErr = err
			}
			return defaultStatus(c.DAG), readErr
		}
		// it is wrong status if the status is running
		status.CorrectRunningStatus()
		return status, nil
	}
	return nil, err
}

func (c *Controller) GetStatusByRequestId(requestId string) (*models.Status, error) {
	db := &database.Database{
		Config: database.DefaultConfig(),
	}
	ret, err := db.FindByRequestId(c.Location, requestId)
	if err != nil {
		return nil, err
	}
	status, _ := c.GetStatus()
	if status != nil && status.RequestId != requestId {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (c *Controller) GetStatusHist(n int) []*models.StatusFile {
	ret := defaultDb().ReadStatusHist(c.Location, n)
	return ret
}

func (c *Controller) UpdateStatus(status *models.Status) error {
	client := sock.Client{Addr: c.SockAddr()}
	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		ss, _ := models.StatusFromJson(res)
		if ss != nil && ss.RequestId == status.RequestId &&
			ss.Status == scheduler.SchedulerStatus_Running {
			return fmt.Errorf("the DAG is running")
		}
	}
	toUpdate, err := defaultDb().FindByRequestId(c.Location, status.RequestId)
	if err != nil {
		return err
	}
	w := &database.Writer{Target: toUpdate.File}
	if err := w.Open(); err != nil {
		return err
	}
	defer w.Close()
	return w.Write(status)
}

func (c *Controller) Save(value string) error {
	// validate
	cl := dag.Loader{}
	_, err := cl.LoadData([]byte(value))
	if err != nil {
		return err
	}
	if !utils.FileExists(c.Location) {
		return fmt.Errorf("the config file %s does not exist", c.Location)
	}
	err = os.WriteFile(c.Location, []byte(value), 0755)
	return err
}

func (c *Controller) Delete() error {
	db := &database.Database{
		Config: database.DefaultConfig(),
	}
	err := db.RemoveAll(c.Location)
	if err != nil {
		return err
	}
	return os.Remove(c.Location)
}

func assertPath(configPath string) error {
	if path.Ext(configPath) != ".yaml" {
		return fmt.Errorf("the config file must be a yaml file with .yaml extension")
	}
	return nil
}

func defaultStatus(d *dag.DAG) *models.Status {
	return models.NewStatus(
		d,
		nil,
		scheduler.SchedulerStatus_None,
		int(models.PidNotRunning), nil, nil)
}

func defaultDb() *database.Database {
	return &database.Database{
		Config: database.DefaultConfig(),
	}
}
