package dag

import (
	"crypto/md5"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/config"

	"github.com/robfig/cron/v3"
)

// Schedule contains the cron expression and the parsed cron schedule.
type Schedule struct {
	Expression string        // Expression is the cron expression.
	Parsed     cron.Schedule // Parsed is the parsed cron schedule.
}

// HandlerOn contains the steps to be executed on different events in the DAG.
type HandlerOn struct {
	Failure *Step
	Success *Step
	Cancel  *Step
	Exit    *Step
}

// MailOn contains the conditions to send mail.
type MailOn struct {
	Failure bool // Failure is the flag to send mail on failure.
	Success bool // Success is the flag to send mail on success.
}

// SmtpConfig contains the SMTP configuration.
type SmtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

// MailConfig contains the mail configuration.
type MailConfig struct {
	From       string
	To         string
	Prefix     string // Prefix is the prefix for the subject of the mail.
	AttachLogs bool   // AttachLogs is the flag to attach the logs in the mail.
}

// DAG contains all information about a workflow.
type DAG struct {
	Location          string        // Location is the absolute path to the DAG file.
	Group             string        // Group is the group name of the DAG. This is optional.
	Name              string        // Name is the name of the DAG. The default is the filename without the extension.
	Schedule          []*Schedule   // Schedule is the start schedule of the DAG.
	StopSchedule      []*Schedule   // StopSchedule is the stop schedule of the DAG.
	RestartSchedule   []*Schedule   // RestartSchedule is the restart schedule of the DAG.
	Description       string        // Description is the description of the DAG. optional.
	Env               []string      // Env contains a list of environment variables to be set before running the DAG.
	LogDir            string        // LogDir is the directory where the logs are stored.
	HandlerOn         HandlerOn     // HandlerOn contains the steps to be executed on different events.
	Steps             []Step        // Steps contains the list of steps in the DAG.
	MailOn            *MailOn       // MailOn contains the conditions to send mail.
	ErrorMail         *MailConfig   // ErrorMail contains the mail configuration for error.
	InfoMail          *MailConfig   // InfoMail contains the mail configuration for info.
	Smtp              *SmtpConfig   // Smtp contains the SMTP configuration.
	Delay             time.Duration // Delay is the delay before starting the DAG.
	RestartWait       time.Duration // RestartWait is the time to wait before restarting the DAG.
	HistRetentionDays int           // HistRetentionDays is the number of days to keep the history.
	Preconditions     []*Condition  // Preconditions contains the conditions to be met before running the DAG.
	MaxActiveRuns     int           // MaxActiveRuns specifies the maximum concurrent steps to run in an execution.
	Params            []string      // Params contains the list of parameters to be passed to the DAG.
	DefaultParams     string        // DefaultParams contains the default parameters to be passed to the DAG.
	MaxCleanUpTime    time.Duration // MaxCleanUpTime is the maximum time to wait for cleanup when the DAG is stopped.
	Tags              []string      // Tags contains the list of tags for the DAG. optional.
}

// setup sets the default values for the DAG.
func (d *DAG) setup() {
	// LogDir is the directory where the logs are stored.
	// It is used to write the stdout and stderr of the steps.
	if d.LogDir == "" {
		d.LogDir = config.Get().LogDir
	}

	// The default history retention days is 30 days.
	// It is the number of days to keep the history.
	// The older history is deleted when the DAG is executed.
	if d.HistRetentionDays == 0 {
		d.HistRetentionDays = defaultHistoryRetentionDays
	}

	// The default max cleanup time is 60 seconds.
	// It is the maximum time to wait for cleanup when the DAG gets a stop signal.
	// If the cleanup takes more than this time, the process of the DAG is killed.
	if d.MaxCleanUpTime == 0 {
		d.MaxCleanUpTime = defaultMaxCleanUpTime
	}

	// set the default working directory for the steps if not set
	dir := path.Dir(d.Location)
	for _, step := range d.Steps {
		step.setup(dir)
	}

	// set the default working directory for the handler steps if not set
	if d.HandlerOn.Exit != nil {
		d.HandlerOn.Exit.setup(dir)
	}
	if d.HandlerOn.Success != nil {
		d.HandlerOn.Success.setup(dir)
	}
	if d.HandlerOn.Failure != nil {
		d.HandlerOn.Failure.setup(dir)
	}
	if d.HandlerOn.Cancel != nil {
		d.HandlerOn.Cancel.setup(dir)
	}
}

// HasTag checks if the DAG has the given tag.
func (d *DAG) HasTag(tag string) bool {
	for _, t := range d.Tags {
		if t == tag {
			return true
		}
	}

	return false
}

// SockAddr returns the unix socket address for the DAG.
// The address is used to communicate with the agent process.
// TODO: It needs to be unique for each process so that multiple processes can run in parallel.
func (d *DAG) SockAddr() string {
	s := strings.ReplaceAll(d.Location, " ", "_")
	name := strings.Replace(path.Base(s), path.Ext(path.Base(s)), "", 1)
	h := md5.New()
	_, _ = h.Write([]byte(s))
	bs := h.Sum(nil)
	return path.Join("/tmp", fmt.Sprintf("@dagu-%s-%x.sock", name, bs))
}

// String implements the Stringer interface.
// It returns the string representation of the DAG.
// TODO: Remove if not needed.
func (d *DAG) String() string {
	ret := "{\n"
	ret = fmt.Sprintf("%s\tName: %s\n", ret, d.Name)
	ret = fmt.Sprintf("%s\tDescription: %s\n", ret, strings.TrimSpace(d.Description))
	ret = fmt.Sprintf("%s\tEnv: %v\n", ret, strings.Join(d.Env, ", "))
	ret = fmt.Sprintf("%s\tLogDir: %v\n", ret, d.LogDir)
	for i, s := range d.Steps {
		ret = fmt.Sprintf("%s\tStep%d: %s\n", ret, i, s.String())
	}
	ret = fmt.Sprintf("%s}\n", ret)
	return ret
}
