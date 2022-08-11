package controller

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/sock"
	"github.com/yohamta/dagu/internal/utils"
)

// Controller is the interface for working with DAGs.
type Controller interface {
	Stop() error
	Start(bin string, workDir string, params string) error
	StartAsync(bin string, workDir string, params string)
	Retry(bin string, workDir string, reqId string) error
	GetStatus() (*models.Status, error)
	GetLastStatus() (*models.Status, error)
	GetStatusByRequestId(requestId string) (*models.Status, error)
	GetStatusHist(n int) []*models.StatusFile
	UpdateStatus(*models.Status) error
	Save(value string) error
}

// GetDAGs returns all DAGs in the config file.
func GetDAGs(dir string) (dags []*DAG, errs []string, err error) {
	dags = []*DAG{}
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
		if utils.MatchExtension(fi.Name(), config.EXTENSIONS) {
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

// NewConfig returns a new config.Config.
func NewConfig(file string) error {
	if err := assertConfigPath(file); err != nil {
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
func RenameConfig(oldConfigPath, newConfigPath string) error {
	if err := assertConfigPath(newConfigPath); err != nil {
		return err
	}
	if err := os.Rename(oldConfigPath, newConfigPath); err != nil {
		return err
	}
	return defaultDb().MoveData(oldConfigPath, newConfigPath)
}

var _ Controller = (*controller)(nil)

type controller struct {
	*config.DAG
}

func New(cfg *config.DAG) Controller {
	return &controller{
		DAG: cfg,
	}
}

func (c *controller) Stop() error {
	client := sock.Client{Addr: c.SockAddr()}
	_, err := client.Request("POST", "/stop")
	return err
}

func (c *controller) StartAsync(bin string, workDir string, params string) {
	go func() {
		err := c.Start(bin, workDir, params)
		utils.LogErr("starting a DAG", err)
	}()
}

func (c *controller) Start(bin string, workDir string, params string) error {
	args := []string{"start"}
	if params != "" {
		args = append(args, fmt.Sprintf("--params=\"%s\"", params))
	}
	args = append(args, c.ConfigPath)
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

func (c *controller) Retry(bin string, workDir string, reqId string) (err error) {
	go func() {
		args := []string{"retry"}
		args = append(args, fmt.Sprintf("--req=%s", reqId))
		args = append(args, c.ConfigPath)
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

func (c *controller) GetStatus() (*models.Status, error) {
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

func (c *controller) GetLastStatus() (*models.Status, error) {
	client := sock.Client{Addr: c.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err == nil {
		return models.StatusFromJson(ret)
	}
	if err == nil || !errors.Is(err, sock.ErrTimeout) {
		status, err := defaultDb().ReadStatusToday(c.ConfigPath)
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

func (c *controller) GetStatusByRequestId(requestId string) (*models.Status, error) {
	db := &database.Database{
		Config: database.DefaultConfig(),
	}
	ret, err := db.FindByRequestId(c.ConfigPath, requestId)
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

func (c *controller) GetStatusHist(n int) []*models.StatusFile {
	ret := defaultDb().ReadStatusHist(c.ConfigPath, n)
	return ret
}

func (c *controller) UpdateStatus(status *models.Status) error {
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
	toUpdate, err := defaultDb().FindByRequestId(c.ConfigPath, status.RequestId)
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

func (c *controller) Save(value string) error {
	// validate
	cl := config.Loader{}
	_, err := cl.LoadData([]byte(value))
	if err != nil {
		return err
	}
	if !utils.FileExists(c.ConfigPath) {
		return fmt.Errorf("the config file %s does not exist", c.ConfigPath)
	}
	err = os.WriteFile(c.ConfigPath, []byte(value), 0755)
	return err
}

func assertConfigPath(configPath string) error {
	if path.Ext(configPath) != ".yaml" {
		return fmt.Errorf("the config file must be a yaml file with .yaml extension")
	}
	return nil
}

func defaultStatus(cfg *config.DAG) *models.Status {
	return models.NewStatus(
		cfg,
		nil,
		scheduler.SchedulerStatus_None,
		int(models.PidNotRunning), nil, nil)
}

func defaultDb() *database.Database {
	return &database.Database{
		Config: database.DefaultConfig(),
	}
}
