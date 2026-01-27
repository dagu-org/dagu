// Package tag provides standardized tag functions for structured logging.
//
// All tag keys use kebab-case naming convention for consistency.
// Use these functions instead of raw strings to ensure consistent
// and type-safe log output across the codebase.
package tag

import (
	"log/slog"
	"time"
)

// Core identification tags

func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Error creates a tag for error objects.
func Error(err any) slog.Attr {
	return slog.Any("err", err)
}

// Step creates a tag for workflow step names.
func Step(name string) slog.Attr {
	return slog.String("step", name)
}

// DAG creates a tag for DAG (workflow) names.
func DAG(name string) slog.Attr {
	return slog.String("dag", name)
}

// SubDAG creates a tag for sub-DAG (sub-workflow) names.
func SubDAG(name string) slog.Attr {
	return slog.String("sub-dag", name)
}

// RunID creates a tag for DAG run execution IDs.
func RunID(id string) slog.Attr {
	return slog.String("run-id", id)
}

// SubRunID creates a tag for sub-DAG run execution IDs.
func SubRunID(id string) slog.Attr {
	return slog.String("sub-run-id", id)
}

// AttemptID creates a tag for specific execution attempt IDs.
func AttemptID(id string) slog.Attr {
	return slog.String("attempt-id", id)
}

// AttemptKey creates a tag for globally unique attempt identifiers.
func AttemptKey(key string) slog.Attr {
	return slog.String("attempt-key", key)
}

// Attempt creates a tag for attempt numbers.
func Attempt(n int) slog.Attr {
	return slog.Int("attempt", n)
}

// RequestID creates a tag for request IDs (for API/external calls).
func RequestID(id string) slog.Attr {
	return slog.String("request-id", id)
}

// WorkerID creates a tag for worker instance IDs.
func WorkerID(id string) slog.Attr {
	return slog.String("worker-id", id)
}

// Namespace creates a tag for namespace names.
func Namespace(ns string) slog.Attr {
	return slog.String("namespace", ns)
}

// Path and file tags

// File creates a tag for file paths.
func File(path string) slog.Attr {
	return slog.String("file", path)
}

// Dir creates a tag for directory paths.
func Dir(path string) slog.Attr {
	return slog.String("dir", path)
}

// Path creates a tag for generic paths (prefer File or Dir when specific).
func Path(path string) slog.Attr {
	return slog.String("path", path)
}

// Execution context tags

// Status creates a tag for execution status values.
func Status(status string) slog.Attr {
	return slog.String("status", status)
}

// Timeout creates a tag for timeout duration values.
func Timeout(d time.Duration) slog.Attr {
	return slog.Duration("timeout", d)
}

// ExitCode creates a tag for process exit codes.
func ExitCode(code int) slog.Attr {
	return slog.Int("exit-code", code)
}

// Signal creates a tag for signal names (e.g., SIGTERM).
func Signal(sig string) slog.Attr {
	return slog.String("signal", sig)
}

// Output creates a tag for output data or variable names.
func Output(out string) slog.Attr {
	return slog.String("output", out)
}

// OutputVar creates a tag for output variable names.
func OutputVar(name string) slog.Attr {
	return slog.String("output-var", name)
}

// MaxRetries creates a tag for maximum retry count.
func MaxRetries(n int) slog.Attr {
	return slog.Int("max-retries", n)
}

// Queue and job tags

// Queue creates a tag for queue names.
func Queue(name string) slog.Attr {
	return slog.String("queue", name)
}

// Job creates a tag for job references.
func Job(ref string) slog.Attr {
	return slog.String("job", ref)
}

// Priority creates a tag for priority values.
func Priority(p int) slog.Attr {
	return slog.Int("priority", p)
}

// Count creates a tag for numeric counts.
func Count(n int) slog.Attr {
	return slog.Int("count", n)
}

// MaxConcurrency creates a tag for maximum concurrency limits.
func MaxConcurrency(n int) slog.Attr {
	return slog.Int("max-concurrency", n)
}

// Alive creates a tag for count of alive/running processes.
func Alive(n int) slog.Attr {
	return slog.Int("alive", n)
}

// Dependency and relationship tags

// Dependency creates a tag for dependency step names.
func Dependency(name string) slog.Attr {
	return slog.String("dependency", name)
}

// Parent creates a tag for parent step or DAG names.
func Parent(name string) slog.Attr {
	return slog.String("parent", name)
}

// Target creates a tag for target DAG or service names.
func Target(name string) slog.Attr {
	return slog.String("target", name)
}

// SubDAGRunDir creates a tag for sub-DAG run directories.
func SubDAGRunDir(dir string) slog.Attr {
	return slog.String("sub-dag-run-dir", dir)
}

// Network and service tags

// Host creates a tag for host addresses.
func Host(host string) slog.Attr {
	return slog.String("host", host)
}

// Port creates a tag for port numbers.
func Port(port int) slog.Attr {
	return slog.Int("port", port)
}

// URL creates a tag for URL values.
func URL(url string) slog.Attr {
	return slog.String("url", url)
}

// Addr creates a tag for network addresses (host:port or socket path).
func Addr(addr string) slog.Attr {
	return slog.String("addr", addr)
}

// Service creates a tag for service names.
func Service(name any) slog.Attr {
	return slog.Any("service", name)
}

// ServiceID creates a tag for service instance IDs.
func ServiceID(id string) slog.Attr {
	return slog.String("service-id", id)
}

// Endpoint creates a tag for API endpoints.
func Endpoint(ep string) slog.Attr {
	return slog.String("endpoint", ep)
}

// Time-related tags

// Interval creates a tag for time intervals.
func Interval(d time.Duration) slog.Attr {
	return slog.Duration("interval", d)
}

// Duration creates a tag for time durations.
func Duration(d time.Duration) slog.Attr {
	return slog.Duration("duration", d)
}

// StartTime creates a tag for start timestamps.
func StartTime(t time.Time) slog.Attr {
	return slog.Time("start-time", t)
}

// EndTime creates a tag for end timestamps.
func EndTime(t time.Time) slog.Attr {
	return slog.Time("end-time", t)
}

// Timestamp creates a tag for generic timestamps.
func Timestamp(t time.Time) slog.Attr {
	return slog.Time("timestamp", t)
}

// Size and capacity tags

// Size creates a tag for size values.
func Size(n int) slog.Attr {
	return slog.Int("size", n)
}

// Length creates a tag for length values.
func Length(n int) slog.Attr {
	return slog.Int("length", n)
}

// MaxSize creates a tag for maximum size limits.
func MaxSize(n int) slog.Attr {
	return slog.Int("max-size", n)
}

// Limit creates a tag for generic limits.
func Limit(n int) slog.Attr {
	return slog.Int("limit", n)
}

// Type and metadata tags

// Type creates a tag for type values.
func Type(t string) slog.Attr {
	return slog.String("type", t)
}

// Name creates a tag for generic names (prefer specific tags like Step, DAG).
func Name(name string) slog.Attr {
	return slog.String("name", name)
}

// ID creates a tag for generic IDs (prefer specific tags like RunID, WorkerID).
func ID(id string) slog.Attr {
	return slog.String("id", id)
}

// Version creates a tag for version values.
func Version(v string) slog.Attr {
	return slog.String("version", v)
}

// Reason creates a tag for reason for an action or state.
func Reason(r string) slog.Attr {
	return slog.String("reason", r)
}

// Email-specific tags

// Subject creates a tag for email subjects.
func Subject(s string) slog.Attr {
	return slog.String("subject", s)
}

// To creates a tag for email recipients.
func To(addr string) slog.Attr {
	return slog.String("to", addr)
}

// From creates a tag for email senders.
func From(addr string) slog.Attr {
	return slog.String("from", addr)
}

// Docker and container tags

// Container creates a tag for container names or IDs.
func Container(name string) slog.Attr {
	return slog.String("container", name)
}

// Image creates a tag for container image names.
func Image(name string) slog.Attr {
	return slog.String("image", name)
}

// PullPolicy creates a tag for image pull policy.
func PullPolicy(policy string) slog.Attr {
	return slog.String("pull-policy", policy)
}

// ShouldPull creates a tag for whether to pull image.
func ShouldPull(pull bool) slog.Attr {
	return slog.Bool("should-pull", pull)
}

// Execution flow tags

// Handler creates a tag for handler names.
func Handler(name string) slog.Attr {
	return slog.String("handler", name)
}

// Operation creates a tag for operations being performed.
func Operation(name string) slog.Attr {
	return slog.String("operation", name)
}

// Phase creates a tag for execution phases.
func Phase(name string) slog.Attr {
	return slog.String("phase", name)
}

// Result creates a tag for operation results.
func Result(r string) slog.Attr {
	return slog.String("result", r)
}

// Configuration tags

// Config creates a tag for configuration names or paths.
func Config(name string) slog.Attr {
	return slog.String("config", name)
}

// Option creates a tag for option names.
func Option(name string) slog.Attr {
	return slog.String("option", name)
}

// Value creates a tag for generic values.
func Value(v any) slog.Attr {
	return slog.Any("value", v)
}

// Key creates a tag for key names.
func Key(k string) slog.Attr {
	return slog.String("key", k)
}

// Pattern creates a tag for patterns (regex, glob, etc.).
func Pattern(p string) slog.Attr {
	return slog.String("pattern", p)
}

// Authentication tags

// User creates a tag for usernames.
func User(name string) slog.Attr {
	return slog.String("user", name)
}

// Token creates a tag for token names or types.
func Token(t string) slog.Attr {
	return slog.String("token", t)
}

// Cert creates a tag for certificate paths.
func Cert(path string) slog.Attr {
	return slog.String("cert", path)
}

// Process tags

// PID creates a tag for process IDs.
func PID(pid int) slog.Attr {
	return slog.Int("pid", pid)
}

// Command creates a tag for commands being executed.
func Command(cmd string) slog.Attr {
	return slog.String("command", cmd)
}

// Args creates a tag for command arguments.
func Args(args []string) slog.Attr {
	return slog.Any("args", args)
}

// Migration and archive tags

// ArchiveDir creates a tag for archive directories.
func ArchiveDir(dir string) slog.Attr {
	return slog.String("archive-dir", dir)
}

// DirsProcessed creates a tag for count of directories processed.
func DirsProcessed(n int) slog.Attr {
	return slog.Int("dirs-processed", n)
}

// FailedRuns creates a tag for count of failed runs.
func FailedRuns(n int) slog.Attr {
	return slog.Int("failed-runs", n)
}

// Trace and observability tags

// TraceID creates a tag for trace IDs for distributed tracing.
func TraceID(id string) slog.Attr {
	return slog.String("trace-id", id)
}

// SpanID creates a tag for span IDs for distributed tracing.
func SpanID(id string) slog.Attr {
	return slog.String("span-id", id)
}

// TraceFlags creates a tag for trace flags.
func TraceFlags(flags any) slog.Attr {
	return slog.Any("trace-flags", flags)
}

// Scheduler-specific tags

// SchedulerID creates a tag for scheduler instance IDs.
func SchedulerID(id string) slog.Attr {
	return slog.String("scheduler-id", id)
}

// Schedule creates a tag for cron schedules.
func Schedule(s string) slog.Attr {
	return slog.String("schedule", s)
}

// NextRun creates a tag for next scheduled run time.
func NextRun(t time.Time) slog.Attr {
	return slog.Time("next-run", t)
}

// JobType creates a tag for the type of scheduled job.
func JobType(t string) slog.Attr {
	return slog.String("job-type", t)
}

// ScheduledTime creates a tag for when a job was scheduled.
func ScheduledTime(t time.Time) slog.Attr {
	return slog.Time("scheduled-time", t)
}

// Worker and poller tags

// PollerID creates a tag for poller instance IDs.
func PollerID(id string) slog.Attr {
	return slog.String("poller-id", id)
}

// PollerIndex creates a tag for poller's index.
func PollerIndex(idx int) slog.Attr {
	return slog.Int("poller-index", idx)
}

// Labels creates a tag for worker labels.
func Labels(labels map[string]string) slog.Attr {
	return slog.Any("labels", labels)
}

// Network binding tags

// BindAddress creates a tag for the address to bind to.
func BindAddress(addr string) slog.Attr {
	return slog.String("bind-address", addr)
}

// AdvertiseAddress creates a tag for the address to advertise.
func AdvertiseAddress(addr string) slog.Attr {
	return slog.String("advertise-address", addr)
}

// InstanceID creates a tag for instance IDs.
func InstanceID(id string) slog.Attr {
	return slog.String("instance-id", id)
}

// LLM and tool calling tags

// Tool creates a tag for LLM tool names.
func Tool(name string) slog.Attr {
	return slog.String("tool", name)
}

// ToolCallID creates a tag for LLM tool call IDs.
func ToolCallID(id string) slog.Attr {
	return slog.String("tool-call-id", id)
}
