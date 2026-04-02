// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

func toCoreDAG(apiDAG *api.DAG) (*core.DAG, error) {
	if apiDAG == nil {
		return nil, fmt.Errorf("remote DAG spec is empty")
	}
	data, err := json.Marshal(apiDAG)
	if err != nil {
		return nil, err
	}
	var dag core.DAG
	if err := json.Unmarshal(data, &dag); err != nil {
		return nil, err
	}
	return &dag, nil
}

func toExecStatus(detail *api.DAGRunDetails) (*exec.DAGRunStatus, error) {
	if detail == nil {
		return nil, fmt.Errorf("remote DAG run details are empty")
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	var status exec.DAGRunStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	status.Root = exec.NewDAGRunRef(detail.RootDAGRunName, detail.RootDAGRunId)
	if detail.ParentDAGRunName != nil && detail.ParentDAGRunId != nil {
		status.Parent = exec.NewDAGRunRef(*detail.ParentDAGRunName, *detail.ParentDAGRunId)
	}
	return &status, nil
}

func validateRemoteStartLikeFlags(ctx *Context) error {
	disallowed := []string{"parent", "root", "worker-id", "attempt-id", "schedule-time"}
	for _, flag := range disallowed {
		if ctx.Command.Flags().Changed(flag) {
			return fmt.Errorf("--%s is only supported in the local context", flag)
		}
	}
	if ctx.Command.Flags().Changed("trigger-type") {
		triggerType, err := ctx.StringParam("trigger-type")
		if err != nil {
			return err
		}
		if triggerType != "" && triggerType != "manual" {
			return fmt.Errorf("--trigger-type=%s is only supported in the local context", triggerType)
		}
	}
	return nil
}

func remoteResolveDAG(ctx *Context, arg string) (*api.DAGFile, error) {
	return ctx.Remote.resolveDAG(ctx, arg)
}

func remoteRunStart(ctx *Context, args []string) error {
	if err := validateRemoteStartLikeFlags(ctx); err != nil {
		return err
	}
	fromRunID, err := ctx.StringParam("from-run-id")
	if err != nil {
		return err
	}
	if fromRunID != "" {
		if len(args) != 1 || ctx.Command.Flags().Changed("params") || ctx.Command.ArgsLenAtDash() != -1 {
			return fmt.Errorf("parameters cannot be provided when using --from-run-id")
		}
		dag, err := remoteResolveDAG(ctx, args[0])
		if err != nil {
			return err
		}
		nameOverride, _ := ctx.StringParam("name")
		resp, err := ctx.Remote.rescheduleDAGRun(ctx, dag.Dag.Name, fromRunID, api.RescheduleDAGRunJSONBody{
			DagName:  stringPtrOrNil(nameOverride),
			DagRunId: nil,
		})
		if err != nil {
			return err
		}
		fmt.Println(resp.DagRunId)
		return nil
	}

	if err := validateStartArgumentSeparator(ctx, args); err != nil {
		return err
	}
	dag, err := remoteResolveDAG(ctx, args[0])
	if err != nil {
		return err
	}
	params := ""
	if ctx.Command.ArgsLenAtDash() >= 0 {
		params = joinNonEmpty(args[1:])
	}
	if flagParams, _ := ctx.StringParam("params"); flagParams != "" {
		params = flagParams
	}
	nameOverride, _ := ctx.StringParam("name")
	runID, _ := ctx.StringParam("run-id")
	tags, err := remoteTagsFromFlag(ctx)
	if err != nil {
		return err
	}
	resp, err := ctx.Remote.startDAG(ctx, dag.FileName, api.ExecuteDAGJSONBody{
		DagName:  stringPtrOrNil(nameOverride),
		DagRunId: stringPtrOrNil(runID),
		Params:   stringPtrOrNil(params),
		Tags:     tags,
	})
	if err != nil {
		return err
	}
	fmt.Println(resp.DagRunId)
	return nil
}

func remoteRunEnqueue(ctx *Context, args []string) error {
	if err := validateRemoteStartLikeFlags(ctx); err != nil {
		return err
	}
	dag, err := remoteResolveDAG(ctx, args[0])
	if err != nil {
		return err
	}
	params := ""
	if ctx.Command.ArgsLenAtDash() >= 0 {
		params = joinNonEmpty(args[1:])
	}
	if flagParams, _ := ctx.StringParam("params"); flagParams != "" {
		params = flagParams
	}
	nameOverride, _ := ctx.StringParam("name")
	runID, _ := ctx.StringParam("run-id")
	queueOverride, _ := ctx.StringParam("queue")
	tags, err := remoteTagsFromFlag(ctx)
	if err != nil {
		return err
	}
	resp, err := ctx.Remote.enqueueDAG(ctx, dag.FileName, api.EnqueueDAGDAGRunJSONBody{
		DagName:  stringPtrOrNil(nameOverride),
		DagRunId: stringPtrOrNil(runID),
		Params:   stringPtrOrNil(params),
		Queue:    stringPtrOrNil(queueOverride),
		Tags:     tags,
	})
	if err != nil {
		return err
	}
	fmt.Println(resp.DagRunId)
	return nil
}

func remoteRunStatus(ctx *Context, args []string) error {
	subRunID, _ := ctx.StringParam("sub-run-id")
	if subRunID != "" {
		return fmt.Errorf("--sub-run-id is not supported for remote contexts")
	}
	dag, err := remoteResolveDAG(ctx, args[0])
	if err != nil {
		return err
	}
	runID, _ := ctx.StringParam("run-id")
	if runID == "" {
		runID = "latest"
	}
	detail, err := ctx.Remote.getDAGRunDetails(ctx, dag.Dag.Name, runID)
	if err != nil {
		return err
	}
	spec, err := ctx.Remote.getDAGRunSpec(ctx, dag.Dag.Name, detail.DagRunId)
	if err != nil {
		return err
	}
	coreDAG, err := toCoreDAG(spec)
	if err != nil {
		return err
	}
	status, err := toExecStatus(detail)
	if err != nil {
		return err
	}
	displayTreeStatus(coreDAG, status)
	return nil
}

func remoteRunHistory(ctx *Context, args []string) error {
	format, err := ctx.StringParam("format")
	if err != nil {
		return err
	}
	if err := validateFormat(format); err != nil {
		return err
	}
	query, limit, err := buildRemoteHistoryQuery(ctx, args)
	if err != nil {
		return err
	}
	runs, err := ctx.Remote.listDAGRuns(ctx, query)
	if err != nil {
		return err
	}
	if len(runs) == 0 {
		fmt.Println("No DAG runs found matching the specified filters.")
		return nil
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}
	statuses := make([]*exec.DAGRunStatus, 0, len(runs))
	for _, run := range runs {
		statuses = append(statuses, &exec.DAGRunStatus{
			Name:         run.Name,
			DAGRunID:     run.DagRunId,
			Status:       core.Status(run.Status),
			StartedAt:    run.StartedAt,
			FinishedAt:   run.FinishedAt,
			QueuedAt:     derefString(run.QueuedAt),
			ScheduleTime: derefString(run.ScheduleTime),
			Params:       derefString(run.Params),
		})
	}
	return renderHistory(format, statuses)
}

func remoteRunStop(ctx *Context, args []string) error {
	dag, err := remoteResolveDAG(ctx, args[0])
	if err != nil {
		return err
	}
	runID, _ := ctx.StringParam("run-id")
	if runID != "" {
		return ctx.Remote.stopDAGRun(ctx, dag.Dag.Name, runID)
	}
	return ctx.Remote.stopAllDAGRuns(ctx, dag.FileName)
}

func remoteRunRetry(ctx *Context, args []string) error {
	runID, _ := ctx.StringParam("run-id")
	stepName, _ := ctx.StringParam("step")
	dag, err := remoteResolveDAG(ctx, args[0])
	if err != nil {
		return err
	}
	return ctx.Remote.retryDAGRun(ctx, dag.Dag.Name, runID, api.RetryDAGRunJSONBody{
		DagRunId: runID,
		StepName: stringPtrOrNil(stepName),
	})
}

func remoteRunRestart(ctx *Context, args []string) error {
	dag, err := remoteResolveDAG(ctx, args[0])
	if err != nil {
		return err
	}
	runID, _ := ctx.StringParam("run-id")
	if runID == "" {
		runID = "latest"
	}
	detail, err := ctx.Remote.getDAGRunDetails(ctx, dag.Dag.Name, runID)
	if err != nil {
		return err
	}
	if core.Status(detail.Status) != core.Running {
		return fmt.Errorf("DAG %s is not running, current status: %s", dag.Dag.Name, core.Status(detail.Status))
	}
	if err := ctx.Remote.stopDAGRun(ctx, dag.Dag.Name, detail.DagRunId); err != nil {
		return err
	}
	if err := waitForRemoteStop(ctx, dag.Dag.Name, detail.DagRunId); err != nil {
		return err
	}
	resp, err := ctx.Remote.rescheduleDAGRun(ctx, dag.Dag.Name, detail.DagRunId, api.RescheduleDAGRunJSONBody{})
	if err != nil {
		return err
	}
	fmt.Println(resp.DagRunId)
	return nil
}

func remoteRunDequeue(ctx *Context, args []string) error {
	queueName := args[0]
	dagRunRef, _ := ctx.StringParam("dag-run")
	if dagRunRef != "" {
		ref, err := exec.ParseDAGRunRef(dagRunRef)
		if err != nil {
			return err
		}
		return ctx.Remote.dequeueDAGRun(ctx, ref.Name, ref.ID)
	}
	items, err := ctx.Remote.listQueueItems(ctx, queueName, api.ListQueueItemsParamsTypeQueued, 1, 1)
	if err != nil {
		return err
	}
	if len(items.Items) == 0 {
		return fmt.Errorf("no dag-run found in queue %s", queueName)
	}
	item := items.Items[0]
	return ctx.Remote.dequeueDAGRun(ctx, item.Name, item.DagRunId)
}

func remoteTagsFromFlag(ctx *Context) (*api.Tags, error) {
	tagsStr, err := ctx.StringParam("tags")
	if err != nil {
		return nil, err
	}
	if tagsStr == "" {
		return nil, nil
	}
	tags := core.NewTags(parseTags(tagsStr))
	if err := core.ValidateTags(tags); err != nil {
		return nil, fmt.Errorf("invalid tags: %w", err)
	}
	tagStrings := tags.Strings()
	converted := make(api.Tags, len(tagStrings))
	copy(converted, tagStrings)
	return &converted, nil
}

func buildRemoteHistoryQuery(ctx *Context, args []string) (remoteHistoryQuery, int, error) {
	var query remoteHistoryQuery
	limit := 100
	if len(args) > 0 {
		if isLikelyLocalDAGArg(args[0]) {
			return query, 0, fmt.Errorf("remote history requires a deployed DAG name, not a local YAML path")
		}
		query.Name = args[0]
	}
	lastDuration, _ := ctx.StringParam("last")
	fromDate, _ := ctx.StringParam("from")
	toDate, _ := ctx.StringParam("to")
	if lastDuration != "" && (fromDate != "" || toDate != "") {
		return query, 0, fmt.Errorf("cannot use --last with --from or --to (conflicting time range specifications)")
	}
	if lastDuration != "" {
		d, err := parseRelativeDuration(lastDuration)
		if err != nil {
			return query, 0, err
		}
		from := time.Now().UTC().Add(-d).Unix()
		query.From = &from
	}
	if fromDate != "" {
		t, err := parseAbsoluteDateTime(fromDate)
		if err != nil {
			return query, 0, err
		}
		from := t.Unix()
		query.From = &from
	}
	if toDate != "" {
		t, err := parseAbsoluteDateTime(toDate)
		if err != nil {
			return query, 0, err
		}
		to := t.Unix()
		query.To = &to
	}
	statusValue, _ := ctx.StringParam("status")
	if statusValue != "" {
		s, err := remoteStatusValue(statusValue)
		if err != nil {
			return query, 0, err
		}
		query.Status = &s
	}
	runID, _ := ctx.StringParam("run-id")
	query.RunID = runID
	tagsStr, _ := ctx.StringParam("tags")
	query.Tags = parseTags(tagsStr)
	limitStr, _ := ctx.StringParam("limit")
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit <= 0 {
			limit = 100
		}
	}
	return query, limit, nil
}

func remoteStatusValue(s string) (int, error) {
	switch s {
	case "running":
		return int(core.Running), nil
	case "succeeded":
		return int(core.Succeeded), nil
	case "failed":
		return int(core.Failed), nil
	case "aborted":
		return int(core.Aborted), nil
	case "queued":
		return int(core.Queued), nil
	case "waiting":
		return int(core.Waiting), nil
	case "none":
		return int(core.NotStarted), nil
	default:
		return 0, fmt.Errorf("invalid status %q", s)
	}
}

func waitForRemoteStop(ctx *Context, name, dagRunID string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := ctx.Remote.getDAGRunDetails(ctx, name, dagRunID)
		if err != nil {
			return err
		}
		if core.Status(detail.Status) != core.Running {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for remote DAG run %s to stop", dagRunID)
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func joinNonEmpty(parts []string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " ")
}
