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

type Controller interface {
	Stop() error
	Start(bin string, workDir string, params string) error
	Retry(bin string, workDir string, reqId string) error
	GetStatus() (*models.Status, error)
	GetLastStatus() (*models.Status, error)
	GetStatusByRequestId(requestId string) (*models.Status, error)
	GetStatusHist(n int) []*models.StatusFile
	UpdateStatus(*models.Status) error
	Save(value string) error
}

func GetDAGs(dir string) (dags []*DAG, errs []string, err error) {
	dags = []*DAG{}
	errs = []string{}
	if !utils.FileExists(dir) {
		errs = append(errs, fmt.Sprintf("invalid DAGs directory: %s", dir))
		return
	}
	fis, err := os.ReadDir(dir)
	utils.LogErr("read DAGs directory", err)
	for _, fi := range fis {
		ex := filepath.Ext(fi.Name())
		if ex == ".yaml" || ex == ".yml" {
			dag, err := fromConfig(filepath.Join(dir, fi.Name()), true)
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

var _ Controller = (*controller)(nil)

type controller struct {
	cfg *config.Config
}

func New(cfg *config.Config) Controller {
	return &controller{
		cfg: cfg,
	}
}

func (c *controller) Stop() error {
	client := sock.Client{Addr: sock.GetSockAddr(c.cfg.ConfigPath)}
	_, err := client.Request("POST", "/stop")
	return err
}

func (c *controller) Start(bin string, workDir string, params string) (err error) {
	go func() {
		args := []string{"start"}
		if params != "" {
			args = append(args, fmt.Sprintf("--params=\"%s\"", params))
		}
		args = append(args, c.cfg.ConfigPath)
		cmd := exec.Command(bin, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
		cmd.Dir = workDir
		cmd.Env = os.Environ()
		defer cmd.Wait()
		err = cmd.Start()
		utils.LogErr("starting a DAG", err)
	}()
	time.Sleep(time.Millisecond * 500)
	return
}

func (c *controller) Retry(bin string, workDir string, reqId string) (err error) {
	go func() {
		args := []string{"retry"}
		args = append(args, fmt.Sprintf("--req=%s", reqId))
		args = append(args, c.cfg.ConfigPath)
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

func (s *controller) GetStatus() (*models.Status, error) {
	client := sock.Client{Addr: sock.GetSockAddr(s.cfg.ConfigPath)}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		} else {
			return defaultStatus(s.cfg), nil
		}
	}
	return models.StatusFromJson(ret)
}

func (s *controller) GetLastStatus() (*models.Status, error) {
	client := sock.Client{Addr: sock.GetSockAddr(s.cfg.ConfigPath)}
	ret, err := client.Request("GET", "/status")
	if err == nil {
		return models.StatusFromJson(ret)
	}
	if err == nil || !errors.Is(err, sock.ErrTimeout) {
		db := database.New(database.DefaultConfig())
		status, err := db.ReadStatusToday(s.cfg.ConfigPath)
		if err != nil {
			var readErr error = nil
			if err != database.ErrNoStatusDataToday && err != database.ErrNoStatusData {
				fmt.Printf("read status failed : %s", err)
				readErr = err
			}
			return defaultStatus(s.cfg), readErr
		}
		// it is wrong status if the status is running
		status.CorrectRunningStatus()
		return status, nil
	}
	return nil, err
}

func (s *controller) GetStatusByRequestId(requestId string) (*models.Status, error) {
	db := database.New(database.DefaultConfig())
	ret, err := db.FindByRequestId(s.cfg.ConfigPath, requestId)
	if err != nil {
		return nil, err
	}
	status, _ := s.GetStatus()
	if status != nil && status.RequestId != requestId {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (s *controller) GetStatusHist(n int) []*models.StatusFile {
	db := database.New(database.DefaultConfig())
	ret := db.ReadStatusHist(s.cfg.ConfigPath, n)
	return ret
}

func (s *controller) UpdateStatus(status *models.Status) error {
	client := sock.Client{Addr: sock.GetSockAddr(s.cfg.ConfigPath)}
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
	db := database.New(database.DefaultConfig())
	toUpdate, err := db.FindByRequestId(s.cfg.ConfigPath, status.RequestId)
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

func (s *controller) Save(value string) error {
	// validate
	cl := config.Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	_, err := cl.LoadData([]byte(value))
	if err != nil {
		return err
	}
	if !utils.FileExists(s.cfg.ConfigPath) {
		return fmt.Errorf("the config file %s does not exist", s.cfg.ConfigPath)
	}
	err = os.WriteFile(s.cfg.ConfigPath, []byte(value), 0755)
	return err
}

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

func RenameConfig(oldConfigPath, newConfigPath string) error {
	if err := assertConfigPath(newConfigPath); err != nil {
		return err
	}
	if err := os.Rename(oldConfigPath, newConfigPath); err != nil {
		return err
	}
	db := database.New(database.DefaultConfig())
	return db.MoveData(oldConfigPath, newConfigPath)
}

func assertConfigPath(configPath string) error {
	if path.Ext(configPath) != ".yaml" {
		return fmt.Errorf("the config file must be a yaml file with .yaml extension")
	}
	return nil
}

func defaultStatus(cfg *config.Config) *models.Status {
	return models.NewStatus(
		cfg,
		nil,
		scheduler.SchedulerStatus_None,
		int(models.PidNotRunning), nil, nil)
}
