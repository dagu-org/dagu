package digraph

import (
	// nolint // gosec
	"crypto/md5"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/robfig/cron/v3"
)

// Constants for configuration defaults
const (
	defaultHistoryRetentionDays = 30
	defaultMaxCleanUpTime       = 60 * time.Second
)

// DAG contains all information about a workflow.
type DAG struct {
	// Location is the absolute path to the DAG file.
	Location string `json:"location,omitempty"`
	// Group is the group name of the DAG. This is optional.
	Group string `json:"group,omitempty"`
	// Name is the name of the DAG. The default is the filename without the extension.
	Name string `json:"name,omitempty"`
	// Dotenv is the path to the dotenv file. This is optional.
	Dotenv []string `json:"dotenv,omitempty"`
	// Tags contains the list of tags for the DAG. This is optional.
	Tags []string `json:"tags,omitempty"`
	// Description is the description of the DAG. This is optional.
	Description string `json:"description,omitempty"`
	// Schedule configuration for starting, stopping, and restarting the DAG.
	Schedule        []Schedule `json:"schedule,omitempty"`
	StopSchedule    []Schedule `json:"stopSchedule,omitempty"`
	RestartSchedule []Schedule `json:"restartSchedule,omitempty"`
	// SkipIfSuccessful indicates whether to skip the DAG if it was successful previously.
	// E.g., when the DAG has already been executed manually before the scheduled time.
	SkipIfSuccessful bool `json:"skipIfSuccessful,omitempty"`
	// Env contains a list of environment variables to be set before running the DAG.
	Env []string `json:"env,omitempty"`
	// LogDir is the directory where the logs are stored.
	LogDir string `json:"logDir,omitempty"`
	// DefaultParams contains the default parameters to be passed to the DAG.
	DefaultParams string `json:"defaultParams,omitempty"`
	// Params contains the list of parameters to be passed to the DAG.
	Params []string `json:"params,omitempty"`
	// Steps contains the list of steps in the DAG.
	Steps []Step `json:"steps,omitempty"`
	// HandlerOn contains the steps to be executed on different events.
	HandlerOn HandlerOn `json:"handlerOn,omitempty"`
	// Preconditions contains the conditions to be met before running the DAG.
	Preconditions []*Condition `json:"preconditions,omitempty"`
	// SMTP contains the SMTP configuration.
	SMTP *SMTPConfig `json:"smtp,omitempty"`
	// ErrorMail contains the mail configuration for errors.
	ErrorMail *MailConfig `json:"errorMail,omitempty"`
	// InfoMail contains the mail configuration for informational messages.
	InfoMail *MailConfig `json:"infoMail,omitempty"`
	// MailOn contains the conditions to send mail.
	MailOn *MailOn `json:"mailOn,omitempty"`
	// Timeout specifies the maximum execution time of the DAG task.
	Timeout time.Duration `json:"timeout,omitempty"`
	// Delay is the delay before starting the DAG.
	Delay time.Duration `json:"delay,omitempty"`
	// RestartWait is the time to wait before restarting the DAG.
	RestartWait time.Duration `json:"restartWait,omitempty"`
	// MaxActiveWorkflows specifies the maximum number of concurrent workflows.
	MaxActiveWorkflows int `json:"maxActiveWorkflows,omitempty"`
	// MaxActiveSteps specifies the maximum concurrent steps to run in an execution.
	MaxActiveSteps int `json:"maxActiveSteps,omitempty"`
	// MaxCleanUpTime is the maximum time to wait for cleanup when the DAG is stopped.
	MaxCleanUpTime time.Duration `json:"maxCleanUpTime,omitempty"`
	// HistRetentionDays is the number of days to keep the history.
	HistRetentionDays int `json:"histRetentionDays,omitempty"`
	// BuildErrors contains any errors encountered while building the DAG.
	BuildErrors []error
}

// FileName returns the filename of the DAG without the extension.
func (d *DAG) FileName() string {
	if d.Location == "" {
		return ""
	}
	return fileutil.TrimYAMLFileExtension(filepath.Base(d.Location))
}

// Schedule contains the cron expression and the parsed cron schedule.
type Schedule struct {
	// Expression is the cron expression.
	Expression string `json:"expression"`
	// Parsed is the parsed cron schedule.
	Parsed cron.Schedule `json:"-"`
}

// MarshalJSON implements the json.Marshaler interface.
func (s Schedule) MarshalJSON() ([]byte, error) {
	// Create a temporary struct for marshaling
	type ScheduleAlias struct {
		Expression string `json:"expression"`
	}

	return json.Marshal(ScheduleAlias{
		Expression: s.Expression,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// and also parses the cron expression to populate the Parsed field.
func (s *Schedule) UnmarshalJSON(data []byte) error {
	// Create a temporary struct for unmarshaling
	type ScheduleAlias struct {
		Expression string `json:"expression"`
	}

	var alias ScheduleAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	s.Expression = alias.Expression

	// Parse the cron expression to populate the Parsed field
	if s.Expression != "" {
		parsed, err := cron.ParseStandard(s.Expression)
		if err != nil {
			return fmt.Errorf("invalid cron expression %q: %w", s.Expression, err)
		}
		s.Parsed = parsed
	}

	return nil
}

// HandlerOn contains the steps to be executed on different events in the DAG.
type HandlerOn struct {
	Failure *Step `json:"failure,omitempty"`
	Success *Step `json:"success,omitempty"`
	Cancel  *Step `json:"cancel,omitempty"`
	Exit    *Step `json:"exit,omitempty"`
}

// MailOn contains the conditions to send mail.
type MailOn struct {
	Failure bool `json:"failure,omitempty"`
	Success bool `json:"success,omitempty"`
}

// SMTPConfig contains the SMTP configuration.
type SMTPConfig struct {
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// MailConfig contains the mail configuration.
type MailConfig struct {
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	AttachLogs bool   `json:"attachLogs,omitempty"`
}

// HandlerType is the type of the handler.
type HandlerType string

const (
	HandlerOnSuccess HandlerType = "onSuccess"
	HandlerOnFailure HandlerType = "onFailure"
	HandlerOnCancel  HandlerType = "onCancel"
	HandlerOnExit    HandlerType = "onExit"
)

func (h HandlerType) String() string {
	return string(h)
}

// ParseHandlerType converts a string to a HandlerType.
func ParseHandlerType(s string) HandlerType {
	return handlerMapping[s]
}

var handlerMapping = map[string]HandlerType{
	"onSuccess": HandlerOnSuccess,
	"onFailure": HandlerOnFailure,
	"onCancel":  HandlerOnCancel,
	"onExit":    HandlerOnExit,
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
func (d *DAG) SockAddr(workflowID string) string {
	if d.Location != "" {
		return SockAddr(d.Location, "")
	}
	return SockAddr(d.Name, workflowID)
}

// SockAddrSub returns the unix socket address for a specific workflow ID.
// This is used to control child workflows.
func (d *DAG) SockAddrSub(workflowID string) string {
	return SockAddr(d.GetName(), workflowID)
}

// GetName returns the name of the DAG.
// If the name is not set, it returns the default name (filename without extension).
func (d *DAG) GetName() string {
	name := d.Name
	if name != "" {
		return name
	}
	return defaultName(d.Location)
}

// String implements the Stringer interface.
// String returns a formatted string representation of the DAG
func (d *DAG) String() string {
	var sb strings.Builder

	sb.WriteString("{\n")
	fmt.Fprintf(&sb, "\tName: %s\n", d.Name)
	fmt.Fprintf(&sb, "\tDescription: %s\n", strings.TrimSpace(d.Description))
	fmt.Fprintf(&sb, "\tParams: %v\n", strings.Join(d.Params, ", "))
	fmt.Fprintf(&sb, "\tLogDir: %v\n", d.LogDir)

	for i, step := range d.Steps {
		fmt.Fprintf(&sb, "\tStep%d: %s\n", i, step.String())
	}

	sb.WriteString("}\n")
	return sb.String()
}

// Validate performs basic validation of the DAG structure
func (d *DAG) Validate() error {
	// Ensure all referenced steps exist
	stepMap := make(map[string]bool)
	for _, step := range d.Steps {
		stepMap[step.Name] = true
	}

	// Check dependencies
	for _, step := range d.Steps {
		for _, dep := range step.Depends {
			if !stepMap[dep] {
				var errList error = ErrorList{
					wrapError("depends", dep, fmt.Errorf("step %s depends on non-existent step", step.Name)),
				}
				return errList
			}
		}
	}

	return nil
}

// initializeDefaults sets the default values for the DAG.
func (d *DAG) initializeDefaults() {
	// Set the name if not set.
	if d.Name == "" {
		d.Name = defaultName(d.Location)
	}

	// Set default history retention days to 30 if not specified.
	if d.HistRetentionDays == 0 {
		d.HistRetentionDays = defaultHistoryRetentionDays
	}

	// Set default max cleanup time to 60 seconds if not specified.
	if d.MaxCleanUpTime == 0 {
		d.MaxCleanUpTime = defaultMaxCleanUpTime
	}

	// Ensure we have a valid working directory
	var workDir = "."
	if d.Location != "" {
		workDir = filepath.Dir(d.Location)
	}

	// Setup steps and handlers with the working directory
	d.setupSteps(workDir)
	d.setupHandlers(workDir)
}

// setupSteps initializes all steps
func (d *DAG) setupSteps(workDir string) {
	for i := range d.Steps {
		d.Steps[i].setup(workDir)
	}
}

// setupHandlers initializes all event handlers
func (d *DAG) setupHandlers(workDir string) {
	handlers := []*Step{
		d.HandlerOn.Exit,
		d.HandlerOn.Success,
		d.HandlerOn.Failure,
		d.HandlerOn.Cancel,
	}

	for _, handler := range handlers {
		if handler != nil {
			handler.setup(workDir)
		}
	}
}

// SockAddr returns the unix socket address for the DAG.
// The address is used to communicate with the agent process.
func SockAddr(name, workflowID string) string {
	maxSocketNameLength := 50 // Maximum length for socket name
	name = fileutil.SafeName(name)
	workflowID = fileutil.SafeName(workflowID)

	// Create MD5 hash of the combined name and workflow ID and take first 8 chars
	combined := name + workflowID
	hashLength := 6
	hash := fmt.Sprintf("%x", md5.Sum([]byte(combined)))[:hashLength] // nolint:gosec

	// Calculate the total length with the full name
	prefix := "@dagu_"
	connector := "_"
	suffix := ".sock"
	totalLen := len(prefix) + len(name) + len(connector) + len(hash) + len(suffix)

	// Truncate name only if the total length exceeds maxSocketNameLength (50)
	if totalLen > maxSocketNameLength {
		// Calculate how much to truncate
		excessLen := totalLen - maxSocketNameLength
		nameLen := len(name) - excessLen
		name = name[:nameLen]
	}

	// Build the socket name
	socketName := prefix + name + connector + hash + suffix

	return filepath.Join("/tmp", socketName)
}
