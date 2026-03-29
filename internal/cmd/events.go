// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/dagu-org/dagu/internal/service/eventstore"
	"github.com/spf13/cobra"
)

func Events() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "events [flags]",
			Short: "Display centralized event log entries",
			Long: `Display centralized event log entries with filtering and formatting options.

Examples:
  dagu events
  dagu events --kind dag_run --type dag.run.failed
  dagu events --dag my-dag --run-id run-123 --last 24h
  dagu events --session-id session-1 --format json
`,
		},
		eventFlags,
		runEvents,
	)
}

var eventFlags = []commandLineFlag{
	historyFromFlag,
	historyToFlag,
	historyLastFlag,
	{
		name:  "kind",
		usage: "Filter by event kind (dag_run, llm_usage)",
	},
	{
		name:  "type",
		usage: "Filter by event type (for example: dag.run.failed, llm.usage.recorded)",
	},
	{
		name:  "dag",
		usage: "Filter by DAG name",
	},
	{
		name:  "run-id",
		usage: "Filter by DAG run ID",
	},
	{
		name:  "attempt-id",
		usage: "Filter by attempt ID",
	},
	{
		name:  "session-id",
		usage: "Filter by session ID",
	},
	{
		name:  "user-id",
		usage: "Filter by user ID",
	},
	{
		name:  "model",
		usage: "Filter by model name",
	},
	{
		name:         "format",
		shorthand:    "f",
		defaultValue: "table",
		usage:        "Output format: table, json, or csv (default: table)",
	},
	{
		name:         "limit",
		shorthand:    "l",
		defaultValue: "100",
		usage:        "Maximum number of results to display (default: 100, max: 1000)",
	},
}

func runEvents(ctx *Context, _ []string) error {
	format, err := ctx.StringParam("format")
	if err != nil {
		return fmt.Errorf("failed to get format parameter: %w", err)
	}
	if err := validateFormat(format); err != nil {
		return err
	}
	if ctx.EventService == nil {
		return fmt.Errorf("event store is not configured")
	}

	filter, err := buildEventFilter(ctx)
	if err != nil {
		return err
	}

	result, err := ctx.EventService.Query(ctx.Context, filter)
	if err != nil {
		return fmt.Errorf("failed to query event log: %w", err)
	}
	if len(result.Entries) == 0 {
		fmt.Println("No events found matching the specified filters.")
		return nil
	}

	switch format {
	case "json":
		return renderEventsJSON(result.Entries)
	case "csv":
		return renderEventsCSV(result.Entries)
	default:
		return renderEventsTable(result.Entries)
	}
}

func buildEventFilter(ctx *Context) (eventstore.QueryFilter, error) {
	filter := eventstore.QueryFilter{}

	kind, err := ctx.StringParam("kind")
	if err != nil {
		return filter, fmt.Errorf("failed to get kind parameter: %w", err)
	}
	eventType, err := ctx.StringParam("type")
	if err != nil {
		return filter, fmt.Errorf("failed to get type parameter: %w", err)
	}
	dagName, err := ctx.StringParam("dag")
	if err != nil {
		return filter, fmt.Errorf("failed to get dag parameter: %w", err)
	}
	runID, err := ctx.StringParam("run-id")
	if err != nil {
		return filter, fmt.Errorf("failed to get run-id parameter: %w", err)
	}
	attemptID, err := ctx.StringParam("attempt-id")
	if err != nil {
		return filter, fmt.Errorf("failed to get attempt-id parameter: %w", err)
	}
	sessionID, err := ctx.StringParam("session-id")
	if err != nil {
		return filter, fmt.Errorf("failed to get session-id parameter: %w", err)
	}
	userID, err := ctx.StringParam("user-id")
	if err != nil {
		return filter, fmt.Errorf("failed to get user-id parameter: %w", err)
	}
	model, err := ctx.StringParam("model")
	if err != nil {
		return filter, fmt.Errorf("failed to get model parameter: %w", err)
	}
	lastDuration, err := ctx.StringParam("last")
	if err != nil {
		return filter, fmt.Errorf("failed to get last parameter: %w", err)
	}
	fromDate, err := ctx.StringParam("from")
	if err != nil {
		return filter, fmt.Errorf("failed to get from parameter: %w", err)
	}
	toDate, err := ctx.StringParam("to")
	if err != nil {
		return filter, fmt.Errorf("failed to get to parameter: %w", err)
	}
	if lastDuration != "" && (fromDate != "" || toDate != "") {
		return filter, fmt.Errorf("cannot use --last with --from or --to (conflicting time range specifications)")
	}
	if lastDuration != "" {
		duration, err := parseRelativeDuration(lastDuration)
		if err != nil {
			return filter, fmt.Errorf("invalid --last value '%s': %w. Valid formats: 7d, 24h, 1w, 30d", lastDuration, err)
		}
		filter.StartTime = time.Now().UTC().Add(-duration)
	} else {
		if fromDate != "" {
			filter.StartTime, err = parseAbsoluteDateTime(fromDate)
			if err != nil {
				return filter, fmt.Errorf("invalid --from date '%s': %w. Expected format: 2006-01-02 or 2006-01-02T15:04:05Z", fromDate, err)
			}
		}
		if toDate != "" {
			filter.EndTime, err = parseAbsoluteDateTime(toDate)
			if err != nil {
				return filter, fmt.Errorf("invalid --to date '%s': %w. Expected format: 2006-01-02 or 2006-01-02T15:04:05Z", toDate, err)
			}
		}
		if !filter.StartTime.IsZero() && !filter.EndTime.IsZero() && filter.StartTime.After(filter.EndTime) {
			return filter, fmt.Errorf("--from date (%s) must be before --to date (%s)", fromDate, toDate)
		}
	}

	limitStr, err := ctx.StringParam("limit")
	if err != nil {
		return filter, fmt.Errorf("failed to get limit parameter: %w", err)
	}
	if limitStr == "" {
		filter.Limit = 100
	} else {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			return filter, fmt.Errorf("invalid --limit value '%s': must be a positive integer", limitStr)
		}
		if limit > 1000 {
			limit = 1000
		}
		filter.Limit = limit
	}

	filter.Kind = eventstore.EventKind(kind)
	filter.Type = eventstore.EventType(eventType)
	filter.DAGName = dagName
	filter.DAGRunID = runID
	filter.AttemptID = attemptID
	filter.SessionID = sessionID
	filter.UserID = userID
	filter.Model = model

	return filter, nil
}

func renderEventsTable(entries []*eventstore.Event) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "TIME\tKIND\tTYPE\tDAG\tRUN\tATTEMPT\tSESSION\tUSER\tMODEL"); err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.OccurredAt.UTC().Format(time.RFC3339),
			entry.Kind,
			entry.Type,
			entry.DAGName,
			entry.DAGRunID,
			entry.AttemptID,
			entry.SessionID,
			entry.UserID,
			entry.Model,
		); err != nil {
			return err
		}
	}
	return w.Flush()
}

func renderEventsJSON(entries []*eventstore.Event) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func renderEventsCSV(entries []*eventstore.Event) error {
	w := csv.NewWriter(os.Stdout)
	if err := w.Write([]string{"occurred_at", "kind", "type", "dag_name", "dag_run_id", "attempt_id", "session_id", "user_id", "model", "status"}); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := w.Write([]string{
			entry.OccurredAt.UTC().Format(time.RFC3339),
			string(entry.Kind),
			string(entry.Type),
			entry.DAGName,
			entry.DAGRunID,
			entry.AttemptID,
			entry.SessionID,
			entry.UserID,
			entry.Model,
			entry.Status,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
