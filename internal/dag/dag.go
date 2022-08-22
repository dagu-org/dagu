package dag

import (
	"crypto/md5"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
	"github.com/robfig/cron/v3"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
	"golang.org/x/sys/unix"
)

// DAG represents a DAG configuration.
type DAG struct {
	Path              string
	Group             string
	Name              string
	Schedule          []cron.Schedule
	ScheduleExp       []string
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
	HistRetentionDays int
	Preconditions     []*Condition
	MaxActiveRuns     int
	Params            []string
	DefaultParams     string
	MaxCleanUpTime    time.Duration
	Tags              []string
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

var EXTENSIONS = []string{".yaml", ".yml"}

func ReadConfig(file string) (string, error) {
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
	s := strings.ReplaceAll(c.Path, " ", "_")
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
	dir := path.Dir(c.Path)
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
		step.Dir = path.Dir(c.Path)
	}
}

type BuildDAGOptions struct {
	headOnly   bool
	parameters string
	noEval     bool
	noSetenv   bool
	defaultEnv map[string]string
}

type builder struct {
	BuildDAGOptions
	baseConfig *DAG
}

type buildStep struct {
	BuildFn  func(def *configDefinition, d *DAG) error
	Headline bool
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func (b *builder) buildFromDefinition(def *configDefinition, baseConfig *DAG) (d *DAG, err error) {
	b.baseConfig = baseConfig

	d = &DAG{}
	d.Init()

	d.Name = def.Name
	if def.Name != "" {
		d.Name = def.Name
	}
	d.Group = def.Group
	d.Description = def.Description
	if def.MailOn != nil {
		d.MailOn = &MailOn{
			Failure: def.MailOn.Failure,
			Success: def.MailOn.Success,
		}
	}
	d.Delay = time.Second * time.Duration(def.DelaySec)
	d.Tags = parseTags(def.Tags)

	for _, bs := range []buildStep{
		{
			BuildFn:  b.buildSchedule,
			Headline: true,
		},
		{
			BuildFn: b.buildEnvVariables,
		},
		{
			BuildFn: b.buildLogdir,
		},
		{
			BuildFn: b.buildParameters,
		},
		{
			BuildFn: b.buildStepsFromDefinition,
		},
		{
			BuildFn: b.buildHandlers,
		},
		{
			BuildFn: b.buildConfig,
		},
		{
			BuildFn: buildSmtpConfigFromDefinition,
		},
		{
			BuildFn: buildErrorMailConfig,
		},
		{
			BuildFn: buildInfoMailConfig,
		},
	} {
		if (b.headOnly && bs.Headline) || !b.headOnly {
			if err = bs.BuildFn(def, d); err != nil {
				return
			}
		}
	}

	return d, nil
}

func (b *builder) buildSchedule(def *configDefinition, d *DAG) (err error) {
	switch (def.Schedule).(type) {
	case string:
		d.ScheduleExp = []string{def.Schedule.(string)}
	case []interface{}:
		items := []string{}
		for _, s := range def.Schedule.([]interface{}) {
			if a, ok := s.(string); ok {
				items = append(items, a)
			} else {
				return fmt.Errorf("schedule must be a string or an array of strings")
			}
		}
		d.ScheduleExp = items
	case nil:
	default:
		return fmt.Errorf("invalid schedule type: %T", def.Schedule)
	}
	d.Schedule, err = parseSchedule(d.ScheduleExp)
	return
}

func (b *builder) buildEnvVariables(def *configDefinition, d *DAG) (err error) {
	var env map[string]string
	env, err = b.loadVariables(def.Env, b.defaultEnv)
	if err == nil {
		d.Env = buildConfigEnv(env)
		if b.baseConfig != nil {
			for _, e := range b.baseConfig.Env {
				key := strings.SplitN(e, "=", 2)[0]
				if _, ok := env[key]; !ok {
					d.Env = append(d.Env, e)
				}
			}
		}
	}
	return
}

func (b *builder) buildLogdir(def *configDefinition, d *DAG) (err error) {
	d.LogDir, err = utils.ParseVariable(def.LogDir)
	return err
}

func (b *builder) buildParameters(def *configDefinition, d *DAG) (err error) {
	d.DefaultParams = def.Params
	p := d.DefaultParams
	if b.parameters != "" {
		p = b.parameters
	}
	var envs []string
	d.Params, envs, err = b.parseParameters(p, !b.noEval)
	if err == nil {
		d.Env = append(d.Env, envs...)
	}
	return
}

func (b *builder) buildHandlers(def *configDefinition, d *DAG) (err error) {
	if def.HandlerOn.Exit != nil {
		def.HandlerOn.Exit.Name = constants.OnExit
		if d.HandlerOn.Exit, err = b.buildStep(d.Env, def.HandlerOn.Exit); err != nil {
			return err
		}
	}

	if def.HandlerOn.Success != nil {
		def.HandlerOn.Success.Name = constants.OnSuccess
		if d.HandlerOn.Success, err = b.buildStep(d.Env, def.HandlerOn.Success); err != nil {
			return
		}
	}

	if def.HandlerOn.Failure != nil {
		def.HandlerOn.Failure.Name = constants.OnFailure
		if d.HandlerOn.Failure, err = b.buildStep(d.Env, def.HandlerOn.Failure); err != nil {
			return
		}
	}

	if def.HandlerOn.Cancel != nil {
		def.HandlerOn.Cancel.Name = constants.OnCancel
		if d.HandlerOn.Cancel, err = b.buildStep(d.Env, def.HandlerOn.Cancel); err != nil {
			return
		}
	}
	return nil
}

func (b *builder) buildConfig(def *configDefinition, d *DAG) (err error) {
	if def.HistRetentionDays != nil {
		d.HistRetentionDays = *def.HistRetentionDays
	}
	d.Preconditions = loadPreCondition(def.Preconditions)
	d.MaxActiveRuns = def.MaxActiveRuns

	if def.MaxCleanUpTimeSec != nil {
		d.MaxCleanUpTime = time.Second * time.Duration(*def.MaxCleanUpTimeSec)
	}
	return nil
}

func (b *builder) parseParameters(value string, eval bool) (
	params []string,
	envs []string,
	err error,
) {
	parser := shellwords.NewParser()
	parser.ParseBacktick = false
	parser.ParseEnv = false

	var parsed []string
	parsed, err = parser.Parse(value)
	if err != nil {
		return
	}

	ret := []string{}
	for i, v := range parsed {
		if eval {
			v, err = utils.ParseCommand(os.ExpandEnv(v))
			if err != nil {
				return nil, nil, err
			}
		}
		if !b.noSetenv {
			if strings.Contains(v, "=") {
				parts := strings.SplitN(v, "=", 2)
				os.Setenv(parts[0], parts[1])
				envs = append(envs, v)
			}
			err = os.Setenv(strconv.Itoa(i+1), v)
			if err != nil {
				return nil, nil, err
			}
		}
		ret = append(ret, v)
	}
	return ret, envs, nil
}

type envVariable struct {
	key string
	val string
}

func (b *builder) loadVariables(strVariables interface{}, defaults map[string]string) (
	map[string]string, error,
) {
	var vals []*envVariable = []*envVariable{}
	for k, v := range defaults {
		vals = append(vals, &envVariable{k, v})
	}

	loadFn := func(a []*envVariable, m map[interface{}]interface{}) ([]*envVariable, error) {
		for k, v := range m {
			if ks, ok := k.(string); ok {
				if vs, ok := v.(string); ok {
					a = append(a, &envVariable{ks, vs})
				} else {
					return a, fmt.Errorf("invalid value for env %s", ks)
				}
			}
		}
		return a, nil
	}

	var err error
	if a, ok := strVariables.(map[interface{}]interface{}); ok {
		vals, err = loadFn(vals, a)
		if err != nil {
			return nil, err
		}
	}

	if a, ok := strVariables.([]interface{}); ok {
		for _, v := range a {
			if aa, ok := v.(map[interface{}]interface{}); ok {
				vals, err = loadFn(vals, aa)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	vars := map[string]string{}
	for _, v := range vals {
		parsed, err := utils.ParseVariable(v.val)
		if err != nil {
			return nil, err
		}
		vars[v.key] = parsed
		if !b.noSetenv {
			err = os.Setenv(v.key, parsed)
			if err != nil {
				return nil, err
			}
		}
	}
	return vars, nil
}

func (b *builder) buildStepsFromDefinition(def *configDefinition, d *DAG) error {
	ret := []*Step{}
	for _, stepDef := range def.Steps {
		step, err := b.buildStep(d.Env, stepDef)
		if err != nil {
			return err
		}
		ret = append(ret, step)
	}
	d.Steps = ret
	return nil
}

func (b *builder) buildStep(variables []string, def *stepDef) (*Step, error) {
	if err := assertStepDef(def); err != nil {
		return nil, err
	}
	step := &Step{}
	step.Name = def.Name
	step.Description = def.Description
	step.CmdWithArgs = def.Command
	step.Command, step.Args = utils.SplitCommand(step.CmdWithArgs, false)
	step.Script = def.Script
	step.Stdout = b.expandEnv(def.Stdout)
	step.Output = def.Output
	step.Dir = b.expandEnv(def.Dir)
	step.Executor = def.Executor
	step.Variables = variables
	step.Depends = def.Depends
	if def.ContinueOn != nil {
		step.ContinueOn.Skipped = def.ContinueOn.Skipped
		step.ContinueOn.Failure = def.ContinueOn.Failure
	}
	if def.RetryPolicy != nil {
		step.RetryPolicy = &RetryPolicy{
			Limit:    def.RetryPolicy.Limit,
			Interval: time.Second * time.Duration(def.RetryPolicy.IntervalSec),
		}
	}
	if def.RepeatPolicy != nil {
		step.RepeatPolicy.Repeat = def.RepeatPolicy.Repeat
		step.RepeatPolicy.Interval = time.Second * time.Duration(def.RepeatPolicy.IntervalSec)
	}
	if def.SignalOnStep != nil {
		sigDef := *def.SignalOnStep
		sig := unix.SignalNum(sigDef)
		if sig == 0 {
			return nil, fmt.Errorf("invalid signal: %s", sigDef)
		}
		step.SignalOnStep = sigDef
	}
	step.MailOnError = def.MailOnError
	step.Preconditions = loadPreCondition(def.Preconditions)
	return step, nil
}

func (b *builder) expandEnv(val string) string {
	if b.noEval {
		return val
	}
	return os.ExpandEnv(val)
}

func buildSmtpConfigFromDefinition(def *configDefinition, d *DAG) (err error) {
	smtp := &SmtpConfig{}
	smtp.Host = def.Smtp.Host
	smtp.Port = def.Smtp.Port
	d.Smtp = smtp
	return nil
}

func buildErrorMailConfig(def *configDefinition, d *DAG) (err error) {
	d.ErrorMail, err = buildMailConfigFromDefinition(def.ErrorMail)
	return
}

func buildInfoMailConfig(def *configDefinition, d *DAG) (err error) {
	d.InfoMail, err = buildMailConfigFromDefinition(def.InfoMail)
	return
}

func buildMailConfigFromDefinition(def mailConfigDef) (*MailConfig, error) {
	d := &MailConfig{}
	d.From = def.From
	d.To = def.To
	d.Prefix = def.Prefix
	return d, nil
}

func buildConfigEnv(vars map[string]string) []string {
	ret := []string{}
	for k, v := range vars {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func loadPreCondition(cond []*conditionDef) []*Condition {
	ret := []*Condition{}
	for _, v := range cond {
		ret = append(ret, &Condition{
			Condition: v.Condition,
			Expected:  v.Expected,
		})
	}
	return ret
}

func parseTags(value string) []string {
	values := strings.Split(value, ",")
	ret := []string{}
	for _, v := range values {
		tag := strings.ToLower(strings.TrimSpace(v))
		if tag != "" {
			ret = append(ret, tag)
		}
	}
	return ret
}

func parseSchedule(values []string) ([]cron.Schedule, error) {
	ret := []cron.Schedule{}
	for _, v := range values {
		sc, err := cronParser.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule: %s", err)
		}
		ret = append(ret, sc)
	}
	return ret, nil
}

func assertDef(def *configDefinition) error {
	if len(def.Steps) == 0 {
		return fmt.Errorf("at least one step must be specified")
	}
	return nil
}

func assertStepDef(def *stepDef) error {
	if def.Name == "" {
		return fmt.Errorf("step name must be specified")
	}
	if def.Command == "" {
		return fmt.Errorf("step command must be specified")
	}
	return nil
}
