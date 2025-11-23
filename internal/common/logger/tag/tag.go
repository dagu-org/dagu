// Package tag provides standardized tag keys for structured logging.
//
// All tag keys use kebab-case naming convention for consistency.
// Use these constants instead of raw strings to ensure consistent
// log output across the codebase.
package tag

// Core identification tags
const (
	// Error is the standard tag for error objects.
	// Always use this instead of "error" for consistency.
	Error = "err"

	// Step identifies a workflow step by name.
	Step = "step"

	// DAG identifies a DAG (workflow) by name.
	DAG = "dag"

	// RunID identifies a DAG run execution.
	RunID = "run-id"

	// AttemptID identifies a specific execution attempt.
	AttemptID = "attempt-id"

	// Attempt identifies an attempt number or reference.
	Attempt = "attempt"

	// RequestID identifies a request (for API/external calls).
	RequestID = "request-id"

	// WorkerID identifies a worker instance.
	WorkerID = "worker-id"
)

// Path and file tags
const (
	// File is the tag for file paths.
	File = "file"

	// Dir is the tag for directory paths.
	Dir = "dir"

	// Path is for generic paths (prefer File or Dir when specific).
	Path = "path"
)

// Execution context tags
const (
	// Status identifies execution status values.
	Status = "status"

	// Timeout identifies timeout duration values.
	Timeout = "timeout"

	// ExitCode identifies process exit codes.
	ExitCode = "exit-code"

	// Signal identifies signal names (e.g., SIGTERM).
	Signal = "signal"

	// Output identifies output data or variable names.
	Output = "output"

	// OutputVar identifies output variable names.
	OutputVar = "output-var"

	// MaxRetries identifies maximum retry count.
	MaxRetries = "max-retries"
)

// Queue and job tags
const (
	// Queue identifies a queue name.
	Queue = "queue"

	// Job identifies a job reference.
	Job = "job"

	// Priority identifies priority values.
	Priority = "priority"

	// Count identifies numeric counts.
	Count = "count"

	// MaxConcurrency identifies maximum concurrency limits.
	MaxConcurrency = "max-concurrency"

	// Alive identifies count of alive/running processes.
	Alive = "alive"
)

// Dependency and relationship tags
const (
	// Dependency identifies a dependency step name.
	Dependency = "dependency"

	// Parent identifies a parent step or DAG.
	Parent = "parent"

	// Target identifies a target DAG or service.
	Target = "target"

	// SubDAGRunDir identifies sub-DAG run directory.
	SubDAGRunDir = "sub-dag-run-dir"
)

// Network and service tags
const (
	// Host identifies host addresses.
	Host = "host"

	// Port identifies port numbers.
	Port = "port"

	// URL identifies URL values.
	URL = "url"

	// Addr identifies network addresses (host:port or socket path).
	Addr = "addr"

	// Service identifies service names.
	Service = "service"

	// ServiceID identifies service instance IDs.
	ServiceID = "service-id"

	// Endpoint identifies API endpoints.
	Endpoint = "endpoint"
)

// Time-related tags
const (
	// Interval identifies time intervals.
	Interval = "interval"

	// Duration identifies time durations.
	Duration = "duration"

	// StartTime identifies start timestamps.
	StartTime = "start-time"

	// EndTime identifies end timestamps.
	EndTime = "end-time"

	// Timestamp identifies generic timestamps.
	Timestamp = "timestamp"
)

// Size and capacity tags
const (
	// Size identifies size values.
	Size = "size"

	// Length identifies length values.
	Length = "length"

	// MaxSize identifies maximum size limits.
	MaxSize = "max-size"

	// Limit identifies generic limits.
	Limit = "limit"
)

// Type and metadata tags
const (
	// Type identifies type values.
	Type = "type"

	// Name identifies generic names (prefer specific tags like Step, DAG).
	Name = "name"

	// ID identifies generic IDs (prefer specific tags like RunID, WorkerID).
	ID = "id"

	// Version identifies version values.
	Version = "version"

	// Reason identifies reason for an action or state.
	Reason = "reason"
)

// Email-specific tags
const (
	// Subject identifies email subjects.
	Subject = "subject"

	// To identifies email recipients.
	To = "to"

	// From identifies email senders.
	From = "from"
)

// Docker and container tags
const (
	// Container identifies container names or IDs.
	Container = "container"

	// Image identifies container image names.
	Image = "image"

	// PullPolicy identifies image pull policy.
	PullPolicy = "pull-policy"

	// ShouldPull identifies whether to pull image.
	ShouldPull = "should-pull"
)

// Execution flow tags
const (
	// Handler identifies handler names.
	Handler = "handler"

	// Action identifies actions being performed.
	Action = "action"

	// Operation identifies operations being performed.
	Operation = "operation"

	// Phase identifies execution phases.
	Phase = "phase"

	// Result identifies operation results.
	Result = "result"
)

// Configuration tags
const (
	// Config identifies configuration names or paths.
	Config = "config"

	// Option identifies option names.
	Option = "option"

	// Value identifies generic values.
	Value = "value"

	// Key identifies key names.
	Key = "key"

	// Pattern identifies patterns (regex, glob, etc.).
	Pattern = "pattern"
)

// Authentication tags
const (
	// User identifies usernames.
	User = "user"

	// Token identifies token names or types.
	Token = "token"

	// Cert identifies certificate paths.
	Cert = "cert"
)

// Process tags
const (
	// PID identifies process IDs.
	PID = "pid"

	// Command identifies commands being executed.
	Command = "command"

	// Args identifies command arguments.
	Args = "args"
)

// Migration and archive tags
const (
	// ArchiveDir identifies archive directories.
	ArchiveDir = "archive-dir"

	// DirsProcessed identifies count of directories processed.
	DirsProcessed = "dirs-processed"

	// FailedRuns identifies count of failed runs.
	FailedRuns = "failed-runs"
)

// Trace and observability tags
const (
	// TraceID identifies trace IDs for distributed tracing.
	TraceID = "trace-id"

	// SpanID identifies span IDs for distributed tracing.
	SpanID = "span-id"

	// TraceFlags identifies trace flags.
	TraceFlags = "trace-flags"
)

// Scheduler-specific tags
const (
	// SchedulerID identifies scheduler instances.
	SchedulerID = "scheduler-id"

	// Schedule identifies cron schedules.
	Schedule = "schedule"

	// NextRun identifies next scheduled run time.
	NextRun = "next-run"
)
