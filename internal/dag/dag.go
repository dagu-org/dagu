package dag

import (
	"crypto/md5"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yohamta/dagu/internal/settings"
)

// DAG represents a DAG configuration.
type DAG struct {
	Location          string
	Group             string
	Name              string
	Schedule          []*Schedule
	StopSchedule      []*Schedule
	RestartSchedule   []*Schedule
	Description       string
	Env               []string
	LogDir            string
	HandlerOn         HandlerOn
	Steps             []*Step
	MailOn            *MailOn
	ErrorMail         *MailConfig
	InfoMail          *MailConfig
	Smtp              *SmtpConfig
	Delay             time.Duration
	RestartWait       time.Duration
	HistRetentionDays int
	Preconditions     []*Condition
	MaxActiveRuns     int
	Params            []string
	DefaultParams     string
	MaxCleanUpTime    time.Duration
	Tags              []string
}

type Schedule struct {
	Expression string
	Parsed     cron.Schedule
}

type HandlerOn struct {
	Failure *Step
	Success *Step
	Cancel  *Step
	Exit    *Step
}

type MailOn struct {
	Failure bool
	Success bool
}

func ReadFile(file string) (string, error) {
	b, err := os.ReadFile(file)
	return string(b), err
}

func (c *DAG) Init() {
	if c.Env == nil {
		c.Env = []string{}
	}
	if c.Steps == nil {
		c.Steps = []*Step{}
	}
	if c.Params == nil {
		c.Params = []string{}
	}
	if c.Preconditions == nil {
		c.Preconditions = []*Condition{}
	}
}

func (c *DAG) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func (c *DAG) SockAddr() string {
	s := strings.ReplaceAll(c.Location, " ", "_")
	name := strings.Replace(path.Base(s), path.Ext(path.Base(s)), "", 1)
	h := md5.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return path.Join("/tmp", fmt.Sprintf("@dagu-%s-%x.sock", name, bs))
}

func (c *DAG) Clone() *DAG {
	ret := *c
	return &ret
}

func (c *DAG) String() string {
	ret := "{\n"
	ret = fmt.Sprintf("%s\tName: %s\n", ret, c.Name)
	ret = fmt.Sprintf("%s\tDescription: %s\n", ret, strings.TrimSpace(c.Description))
	ret = fmt.Sprintf("%s\tEnv: %v\n", ret, strings.Join(c.Env, ", "))
	ret = fmt.Sprintf("%s\tLogDir: %v\n", ret, c.LogDir)
	for i, s := range c.Steps {
		ret = fmt.Sprintf("%s\tStep%d: %v\n", ret, i, s)
	}
	ret = fmt.Sprintf("%s}\n", ret)
	return ret
}

func (c *DAG) setup() {
	if c.LogDir == "" {
		c.LogDir = path.Join(settings.MustGet(settings.SETTING__LOGS_DIR), "dags")
	}
	if c.HistRetentionDays == 0 {
		c.HistRetentionDays = 30
	}
	if c.MaxCleanUpTime == 0 {
		c.MaxCleanUpTime = time.Second * 60
	}
	dir := path.Dir(c.Location)
	for _, step := range c.Steps {
		c.setupStep(step, dir)
	}
	if c.HandlerOn.Exit != nil {
		c.setupStep(c.HandlerOn.Exit, dir)
	}
	if c.HandlerOn.Success != nil {
		c.setupStep(c.HandlerOn.Success, dir)
	}
	if c.HandlerOn.Failure != nil {
		c.setupStep(c.HandlerOn.Failure, dir)
	}
	if c.HandlerOn.Cancel != nil {
		c.setupStep(c.HandlerOn.Cancel, dir)
	}
}

func (c *DAG) setupStep(step *Step, defaultDir string) {
	if step.Dir == "" {
		step.Dir = path.Dir(c.Location)
	}
}
