package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/spf13/cobra"
)

func History() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "history [flags] [DAG name]",
			Short: "Display DAG run history",
			Long: `Display execution history of DAG runs with filtering and formatting options.

This command retrieves and displays historical DAG run information from local storage,
allowing you to query by various criteria including time range, status, tags, and run ID.

Date/Time Filtering:
  --from, --to      Absolute date range in UTC (formats: 2006-01-02 or 2006-01-02T15:04:05Z)
  --last            Relative time period (examples: 7d, 24h, 1w, 30d)
                    Note: --last cannot be combined with --from or --to

Status Filtering:
  --status          Filter by execution status (running, succeeded, failed, aborted, skipped, waiting, none)

Other Filters:
  --tags            Filter by DAG tags (comma-separated, AND logic)
  --run-id          Filter by run ID (partial match supported)

Output Control:
  --format          Output format: table (default) or json
  --limit           Maximum number of results to display (default: 100, max: 1000)

Default Behavior:
  Without date filters, displays runs from the last 30 days, newest first.
  All timestamps are displayed in UTC timezone.

Examples:
  dagu history                                # All runs from last 30 days
  dagu history my-dag                         # All runs for specific DAG
  dagu history --from 2026-01-01              # Runs since date
  dagu history --last 7d                      # Last 7 days
  dagu history --status failed                # Only failed runs
  dagu history --format json                  # JSON output
  dagu history --tags "prod,critical"         # Filter by tags (AND logic)
  dagu history --limit 50                     # Limit to 50 results
  dagu history my-dag --status failed --last 24h  # Combined filters
`,
			Args: cobra.MaximumNArgs(1),
		},
		historyFlags,
		runHistory,
	)
}

var historyFlags = []commandLineFlag{
	historyFromFlag,
	historyToFlag,
	historyLastFlag,
	historyStatusFlag,
	historyRunIDFlag,
	historyTagsFlag,
	historyFormatFlag,
	historyLimitFlag,
}

func runHistory(ctx *Context, args []string) error {
	// Parse and validate flags
	opts, err := buildHistoryOptions(ctx, args)
	if err != nil {
		return err
	}

	// Query DAG run history
	statuses, err := ctx.DAGRunStore.ListStatuses(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to query DAG run history: %w", err)
	}

	// Handle empty results
	if len(statuses) == 0 {
		fmt.Println("No DAG runs found matching the specified filters.")
		return nil
	}

	// Get output format
	format, err := ctx.StringParam("format")
	if err != nil {
		return fmt.Errorf("failed to get format parameter: %w", err)
	}

	// Render output based on format
	switch format {
	case "json":
		return renderHistoryJSON(statuses)
	case "table":
		return renderHistoryTable(statuses)
	default:
		return fmt.Errorf("invalid format '%s'. Valid formats: table, json", format)
	}
}

// buildHistoryOptions constructs query options from command-line flags.
func buildHistoryOptions(ctx *Context, args []string) ([]exec.ListDAGRunStatusesOption, error) {
	var opts []exec.ListDAGRunStatusesOption

	// DAG name from positional argument
	if len(args) > 0 {
		dagName, err := extractDAGName(ctx, args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to extract DAG name: %w", err)
		}
		opts = append(opts, exec.WithExactName(dagName))
	}

	// Date range: --last takes precedence over --from/--to
	lastDuration, err := ctx.StringParam("last")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'last' parameter: %w", err)
	}

	fromDate, err := ctx.StringParam("from")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'from' parameter: %w", err)
	}

	toDate, err := ctx.StringParam("to")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'to' parameter: %w", err)
	}

	// Validate conflicting flags
	if lastDuration != "" && (fromDate != "" || toDate != "") {
		return nil, fmt.Errorf("cannot use --last with --from or --to (conflicting time range specifications)")
	}

	if lastDuration != "" {
		// Parse relative duration
		duration, err := parseRelativeDuration(lastDuration)
		if err != nil {
			return nil, fmt.Errorf("invalid --last value '%s': %w. Valid formats: 7d, 24h, 1w, 30d", lastDuration, err)
		}
		fromTime := time.Now().UTC().Add(-duration)
		opts = append(opts, exec.WithFrom(exec.NewUTC(fromTime)))
	} else {
		// Parse absolute dates
		if fromDate != "" {
			fromTime, err := parseAbsoluteDateTime(fromDate)
			if err != nil {
				return nil, fmt.Errorf("invalid --from date '%s': %w. Expected format: 2006-01-02 or 2006-01-02T15:04:05Z", fromDate, err)
			}
			opts = append(opts, exec.WithFrom(exec.NewUTC(fromTime)))
		} else if toDate == "" {
			// Default: last 30 days if no date filters specified
			defaultFrom := time.Now().UTC().AddDate(0, 0, -30)
			opts = append(opts, exec.WithFrom(exec.NewUTC(defaultFrom)))
		}

		if toDate != "" {
			toTime, err := parseAbsoluteDateTime(toDate)
			if err != nil {
				return nil, fmt.Errorf("invalid --to date '%s': %w. Expected format: 2006-01-02 or 2006-01-02T15:04:05Z", toDate, err)
			}
			opts = append(opts, exec.WithTo(exec.NewUTC(toTime)))
		}

		// Validate from < to
		if fromDate != "" && toDate != "" {
			fromTime, _ := parseAbsoluteDateTime(fromDate)
			toTime, _ := parseAbsoluteDateTime(toDate)
			if fromTime.After(toTime) {
				return nil, fmt.Errorf("--from date (%s) must be before --to date (%s)", fromDate, toDate)
			}
		}
	}

	// Status filter
	statusStr, err := ctx.StringParam("status")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'status' parameter: %w", err)
	}
	if statusStr != "" {
		status, err := parseStatus(statusStr)
		if err != nil {
			return nil, err
		}
		opts = append(opts, exec.WithStatuses([]core.Status{status}))
	}

	// Run ID filter
	runID, err := ctx.StringParam("run-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'run-id' parameter: %w", err)
	}
	if runID != "" {
		opts = append(opts, exec.WithDAGRunID(runID))
	}

	// Tags filter
	tagsStr, err := ctx.StringParam("tags")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'tags' parameter: %w", err)
	}
	if tagsStr != "" {
		tags := parseTags(tagsStr)
		if len(tags) > 0 {
			opts = append(opts, exec.WithTags(tags))
		}
	}

	// Limit (default: 100, max: 1000)
	limitStr, err := ctx.StringParam("limit")
	if err != nil {
		return nil, fmt.Errorf("failed to get 'limit' parameter: %w", err)
	}
	limit := 100 // default
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			return nil, fmt.Errorf("invalid --limit value '%s': must be a positive integer", limitStr)
		}
		if parsedLimit > 1000 {
			fmt.Fprintf(os.Stderr, "Warning: limit capped at 1000 (requested: %d)\n", parsedLimit)
			limit = 1000
		} else {
			limit = parsedLimit
		}
	}
	// Note: The store enforces a max of 1000, but we set it explicitly here
	if limit > 0 {
		opts = append(opts, exec.WithLimit(limit))
	}

	return opts, nil
}

// parseRelativeDuration parses relative time duration strings like "7d", "24h", "1w".
func parseRelativeDuration(s string) (time.Duration, error) {
	// Match pattern: number followed by unit (d=days, h=hours, w=weeks)
	re := regexp.MustCompile(`^(\d+)([dhw])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid format (expected: 7d, 24h, 1w)")
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number: %w", err)
	}

	unit := matches[2]
	switch unit {
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid unit '%s' (valid: h, d, w)", unit)
	}
}

// parseAbsoluteDateTime parses absolute date/time strings in UTC.
// Supported formats: "2006-01-02" (midnight UTC) and "2006-01-02T15:04:05Z" (RFC3339).
func parseAbsoluteDateTime(s string) (time.Time, error) {
	// Try RFC3339 format first
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t.UTC(), nil
	}

	// Try date-only format (treat as midnight UTC)
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	// Try datetime format without timezone (treat as UTC)
	t, err = time.Parse("2006-01-02T15:04:05", s)
	if err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
	}

	return time.Time{}, fmt.Errorf("unsupported date format")
}

// parseStatus converts status string to core.Status with validation.
func parseStatus(s string) (core.Status, error) {
	// Normalize input (lowercase, trim)
	s = strings.ToLower(strings.TrimSpace(s))

	// Map of valid status strings to core.Status
	validStatuses := map[string]core.Status{
		"not_started":         core.NotStarted,
		"notstarted":          core.NotStarted,
		"running":             core.Running,
		"succeeded":           core.Succeeded,
		"success":             core.Succeeded, // Alias
		"failed":              core.Failed,
		"failure":             core.Failed, // Alias
		"aborted":             core.Aborted,
		"canceled":            core.Aborted, // Alias
		"cancelled":           core.Aborted, // Alias
		"cancel":              core.Aborted, // Alias
		"queued":              core.Queued,
		"partially_succeeded": core.PartiallySucceeded,
		"partiallysucceeded":  core.PartiallySucceeded,
		"waiting":             core.Waiting,
		"rejected":            core.Rejected,
	}

	status, ok := validStatuses[s]
	if !ok {
		return core.NotStarted, fmt.Errorf("invalid status '%s'. Valid values: running, succeeded, failed, aborted, queued, waiting, rejected, not_started, partially_succeeded", s)
	}

	return status, nil
}

// parseTags splits comma-separated tags and trims whitespace.
func parseTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

// renderHistoryTable displays DAG run history as an aligned table.
func renderHistoryTable(statuses []*exec.DAGRunStatus) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	fmt.Fprintln(w, "DAG NAME\tRUN ID\tSTATUS\tSTARTED (UTC)\tDURATION\tPARAMS")

	// Rows
	for _, status := range statuses {
		dagName := status.Name
		runID := status.DAGRunID
		statusText := formatStatusText(status.Status)
		startedAt := formatTimestamp(status.StartedAt)
		duration := formatDuration(status)
		params := formatParams(status.Params)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			dagName,
			runID,
			statusText,
			startedAt,
			duration,
			params,
		)
	}

	return nil
}

// renderHistoryJSON displays DAG run history as JSON.
func renderHistoryJSON(statuses []*exec.DAGRunStatus) error {
	// Convert to a more user-friendly JSON structure
	type historyEntry struct {
		Name       string   `json:"name"`
		DAGRunID   string   `json:"dagRunId"`
		Status     string   `json:"status"`
		StartedAt  string   `json:"startedAt,omitempty"`
		FinishedAt string   `json:"finishedAt,omitempty"`
		Duration   string   `json:"duration,omitempty"`
		Params     string   `json:"params,omitempty"`
		Tags       []string `json:"tags,omitempty"`
		WorkerID   string   `json:"workerId,omitempty"`
		Error      string   `json:"error,omitempty"`
	}

	entries := make([]historyEntry, 0, len(statuses))
	for _, status := range statuses {
		entry := historyEntry{
			Name:      status.Name,
			DAGRunID:  status.DAGRunID,
			Status:    status.Status.String(),
			StartedAt: status.StartedAt,
			Params:    status.Params,
			Tags:      status.Tags,
			WorkerID:  status.WorkerID,
			Error:     status.Error,
		}

		// Add finished time if available
		if status.FinishedAt != "" {
			entry.FinishedAt = status.FinishedAt
		}

		// Calculate duration
		entry.Duration = formatDuration(status)

		entries = append(entries, entry)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// formatStatusText converts core.Status to human-readable text.
func formatStatusText(status core.Status) string {
	switch status {
	case core.NotStarted:
		return "Not Started"
	case core.Running:
		return "Running"
	case core.Succeeded:
		return "Succeeded"
	case core.Failed:
		return "Failed"
	case core.Aborted:
		return "Aborted"
	case core.Queued:
		return "Queued"
	case core.PartiallySucceeded:
		return "Partially Succeeded"
	case core.Waiting:
		return "Waiting"
	case core.Rejected:
		return "Rejected"
	default:
		return status.String()
	}
}

// formatTimestamp formats a timestamp string for display in UTC.
// Handles empty strings and invalid formats gracefully.
func formatTimestamp(ts string) string {
	if ts == "" || ts == "-" {
		return "-"
	}

	// Parse the timestamp (expecting RFC3339 or "2006-01-02 15:04:05")
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try alternative format
		t, err = time.Parse("2006-01-02 15:04:05", ts)
		if err != nil {
			return ts // Return as-is if unparseable
		}
	}

	// Format in UTC with standard display format
	return t.UTC().Format("2006-01-02 15:04:05")
}

// formatDuration calculates and formats the duration of a DAG run.
// For running DAGs, shows elapsed time. For completed DAGs, shows total duration.
func formatDuration(status *exec.DAGRunStatus) string {
	if status.StartedAt == "" {
		return "-"
	}

	startTime, err := time.Parse(time.RFC3339, status.StartedAt)
	if err != nil {
		// Try alternative format
		startTime, err = time.Parse("2006-01-02 15:04:05", status.StartedAt)
		if err != nil {
			return "-"
		}
	}

	var endTime time.Time
	if status.FinishedAt != "" {
		endTime, err = time.Parse(time.RFC3339, status.FinishedAt)
		if err != nil {
			endTime, err = time.Parse("2006-01-02 15:04:05", status.FinishedAt)
			if err != nil {
				endTime = time.Now().UTC()
			}
		}
	} else {
		// For running DAGs, use current time
		endTime = time.Now().UTC()
	}

	duration := endTime.Sub(startTime)
	return formatDurationHuman(duration)
}

// formatDurationHuman formats a duration in a human-readable way.
// Examples: "5s", "2m30s", "1h5m", "2d3h"
func formatDurationHuman(d time.Duration) string {
	if d < 0 {
		return "-"
	}

	// Handle sub-second durations
	if d < time.Second {
		return "< 1s"
	}

	// Calculate components
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	// Format based on magnitude
	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}

	if minutes > 0 {
		if seconds > 0 {
			return fmt.Sprintf("%dm%ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", seconds)
}

// formatParams formats parameters for table display.
// Truncates long parameter strings with ellipsis.
func formatParams(params string) string {
	if params == "" {
		return "-"
	}

	// Truncate if too long (max 40 chars for table readability)
	maxLen := 40
	if len(params) > maxLen {
		return params[:maxLen-3] + "..."
	}

	return params
}
