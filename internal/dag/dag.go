package dag

import (
	"crypto/md5"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/util"

	"github.com/robfig/cron/v3"
)

// Schedule contains the cron expression and the parsed cron schedule.
type Schedule struct {
	// Expression is the cron expression.
	Expression string `json:"Expression"`
	// Parsed is the parsed cron schedule.
	Parsed cron.Schedule `json:"-"`
}

// HandlerOn contains the steps to be executed on different events in the DAG.
type HandlerOn struct {
	Failure *Step `json:"Failure"` // Failure is the step to be executed on failure.
	Success *Step `json:"Success"` // Success is the step to be executed on success.
	Cancel  *Step `json:"Cancel"`  // Cancel is the step to be executed on cancel.
	Exit    *Step `json:"Exit"`    // Exit is the step to be executed on exit.
}

// MailOn contains the conditions to send mail.
type MailOn struct {
	Failure bool `json:"Failure"` // Failure is the flag to send mail on failure.
	Success bool `json:"Success"` // Success is the flag to send mail on success.
}

// SmtpConfig contains the SMTP configuration.
type SmtpConfig struct {
	Host     string `json:"Host"`
	Port     string `json:"Port"`
	Username string `json:"Username"`
	Password string `json:"Password"`
}

// MailConfig contains the mail configuration.
type MailConfig struct {
	From       string `json:"From"`
	To         string `json:"To"`
	Prefix     string `json:"Prefix"`     // Prefix is the prefix for the subject of the mail.
	AttachLogs bool   `json:"AttachLogs"` // AttachLogs is the flag to attach the logs in the mail.
}

// DAG contains all information about a workflow.
type DAG struct {
	Location    string   `json:"Location"`    // Location is the absolute path to the DAG file.
	Group       string   `json:"Group"`       // Group is the group name of the DAG. This is optional.
	Name        string   `json:"Name"`        // Name is the name of the DAG. The default is the filename without the extension.
	Tags        []string `json:"Tags"`        // Tags contains the list of tags for the DAG. optional.
	Description string   `json:"Description"` // Description is the description of the DAG. optional.

	// Schedule configuration.
	// This is used by the scheduler to start / stop / restart the DAG.
	Schedule        []*Schedule `json:"Schedule"`        // Schedule is the start schedule of the DAG.
	StopSchedule    []*Schedule `json:"StopSchedule"`    // StopSchedule is the stop schedule of the DAG.
	RestartSchedule []*Schedule `json:"RestartSchedule"` // RestartSchedule is the restart schedule of the DAG.

	// Env contains a list of environment variables to be set before running the DAG.
	Env []string `json:"Env"`

	// LogDir is the directory where the logs are stored.
	// The actual log directory is LogDir + Name (with invalid characters replaced with '_').
	LogDir string `json:"LogDir"`

	// Paramerter configuration.
	// The DAG definition contains only DefaultParams. Params are automatically set by the DAG loader.
	DefaultParams string   `json:"DefaultParams"` // DefaultParams contains the default parameters to be passed to the DAG.
	Params        []string `json:"Params"`        // Params contains the list of parameters to be passed to the DAG.

	// Commands configuration to be executed in the DAG.
	// Steps represents the nodes in the DAG.
	Steps     []Step    `json:"Steps"`     // Steps contains the list of steps in the DAG.
	HandlerOn HandlerOn `json:"HandlerOn"` // HandlerOn contains the steps to be executed on different events.

	// Preconditions contains the conditions to be met before running the DAG.
	// If the conditions are not met, the whole DAG is skipped.
	Preconditions []*Condition `json:"Preconditions"`

	// Mail notification configuration.
	// MailOn contains the conditions to send mail.
	// Smtp contains the SMTP configuration.
	// If you don't want to repeat the SMTP configuration for each DAG, you can set it in the base configuration.
	Smtp      *SmtpConfig `json:"Smtp"`      // Smtp contains the SMTP configuration.
	ErrorMail *MailConfig `json:"ErrorMail"` // ErrorMail contains the mail configuration for error.
	InfoMail  *MailConfig `json:"InfoMail"`  // InfoMail contains the mail configuration for info.
	MailOn    *MailOn     `json:"MailOn"`    // MailOn contains the conditions to send mail.

	// Misc configuration for DAG execution.
	Delay             time.Duration `json:"Delay"`             // Delay is the delay before starting the DAG.
	RestartWait       time.Duration `json:"RestartWait"`       // RestartWait is the time to wait before restarting the DAG.
	MaxActiveRuns     int           `json:"MaxActiveRuns"`     // MaxActiveRuns specifies the maximum concurrent steps to run in an execution.
	MaxCleanUpTime    time.Duration `json:"MaxCleanUpTime"`    // MaxCleanUpTime is the maximum time to wait for cleanup when the DAG is stopped.
	HistRetentionDays int           `json:"HistRetentionDays"` // HistRetentionDays is the number of days to keep the history.
}

// HandlerType is the type of the handler.
type HandlerType string

const (
	HandlerOnSuccess HandlerType = "onSuccess"
	HandlerOnFailure HandlerType = "onFailure"
	HandlerOnCancel  HandlerType = "onCancel"
	HandlerOnExit    HandlerType = "onExit"
)

func (e HandlerType) String() string {
	return string(e)
}

// ParseHandlerType converts a string to a HandlerType.
func ParseHandlerType(s string) HandlerType {
	return nameToHandlerType[s]
}

var (
	nameToHandlerType = map[string]HandlerType{
		"onSuccess": HandlerOnSuccess,
		"onFailure": HandlerOnFailure,
		"onCancel":  HandlerOnCancel,
		"onExit":    HandlerOnExit,
	}
)

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
	for i := range d.Steps {
		d.Steps[i].setup(dir)
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

// GetLogDir returns the log directory for the DAG.
// Log directory is the directory where the execution logs are stored.
// It is DAG.LogDir + DAG.Name (with invalid characters replaced with '_').
func (d *DAG) GetLogDir() string {
	return path.Join(d.LogDir, util.ValidFilename(d.Name))
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
