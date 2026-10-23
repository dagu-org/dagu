// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"

	"github.com/dagucloud/dagu/v2/internal/ir"
)

// Bounds keep structured run outputs small enough for tool results. Outputs are
// explicitly published by workflows and the read already passed API
// authorization, so retention is verbatim; entries beyond the budgets are
// omitted intact and reported, never cut mid-value.
const (
	runOutputsMaxItems = 50
	// Per-entry cap on key plus value bytes; larger entries are skipped whole.
	runOutputsMaxValueBytes = 16 * 1024
	// Total cap on retained key plus value bytes.
	runOutputsMaxTotalBytes = 64 * 1024
)

// attachBoundedRunOutputs adds the run's collected outputs to data. An outputs
// failure never hides the run's terminal state; it is recorded as outputsError
// alongside the details the read already resolved.
func (svc *Service) attachBoundedRunOutputs(ctx context.Context, data map[string]any, addr runAddress) {
	if svc == nil || svc.api == nil {
		maps.Copy(data, runOutputsEnvelope(nil, nil))
		return
	}
	if addr.subRunID == "" {
		addr.name, _ = data["name"].(string)
		addr.dagRunID, _ = data["dagRunId"].(string)
	} else {
		addr.subRunID, _ = data["dagRunId"].(string)
	}
	attemptID, _ := data["attemptId"].(string)
	maps.Copy(data, svc.outputsForAttempt(ctx, addr, attemptID))
}

func (svc *Service) outputsForAttempt(ctx context.Context, addr runAddress, attemptID string) map[string]any {
	outputs, err := svc.api.GetDAGRunOutputsData(ctx, ir.NewDAGRunRef(addr.name, addr.dagRunID), addr.subRunID)
	if err == nil && outputs != nil && attemptID != "" && outputs.Metadata.AttemptID != attemptID {
		return runOutputsEnvelope(nil, fmt.Errorf("run attempt changed while reading outputs; read the run again"))
	}
	return runOutputsEnvelope(outputs, err)
}

// runOutputsEnvelope describes retained outputs and why anything is missing.
// Status is ok, empty, unavailable, or error; truncation is a separate flag so
// omitted entries are always reported.
func runOutputsEnvelope(outputs *ir.DAGRunOutputs, readErr error) map[string]any {
	if readErr != nil {
		return map[string]any{
			"outputs":       map[string]string{},
			"outputsStatus": "error",
			"outputsError":  readErr.Error(),
		}
	}
	if outputs == nil {
		return map[string]any{
			"outputs":       map[string]string{},
			"outputsStatus": "unavailable",
		}
	}
	retained, truncated := boundedRunOutputs(outputs.Outputs)
	status := "ok"
	if len(outputs.Outputs) == 0 {
		status = "empty"
	}
	envelope := map[string]any{
		"outputs":       retained,
		"outputsStatus": status,
	}
	if truncated {
		envelope["outputsTruncated"] = true
	}
	return envelope
}

// withResourceRunOutputs adds the shared outputs envelope to a raw run or
// sub-run details response without changing its existing shape. The address is
// canonicalized to the concrete IDs the response resolved, so a latest alias
// reads the run it identified.
func (svc *Service) withResourceRunOutputs(ctx context.Context, data any, addr runAddress) any {
	run, err := dagRunDetailsFromResponse(data)
	if err != nil {
		return data
	}
	name := string(run.Name)
	dagRunID := string(run.DagRunId)
	if addr.subRunID == "" {
		if name != "" {
			addr.name = name
		}
		if dagRunID != "" {
			addr.dagRunID = dagRunID
		}
	} else if dagRunID != "" {
		addr.subRunID = dagRunID
	}
	attemptID := ""
	if run.AttemptId != nil {
		attemptID = *run.AttemptId
	}
	envelope := svc.outputsForAttempt(ctx, addr, attemptID)
	raw, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var shaped map[string]any
	if err := json.Unmarshal(raw, &shaped); err != nil {
		return data
	}
	maps.Copy(shaped, envelope)
	return shaped
}

// boundedRunOutputs retains named outputs verbatim within item and byte
// budgets. Keys are sorted so omission is deterministic; an entry that would
// exceed the budgets is skipped intact while later small entries are still
// considered.
func boundedRunOutputs(outputs map[string]string) (map[string]string, bool) {
	keys := make([]string, 0, len(outputs))
	for key := range outputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	retained := make(map[string]string, min(len(keys), runOutputsMaxItems))
	truncated := false
	total := 0
	for _, key := range keys {
		value := outputs[key]
		entryBytes := len(key) + len(value)
		if entryBytes > runOutputsMaxValueBytes || total+entryBytes > runOutputsMaxTotalBytes {
			truncated = true
			continue
		}
		if len(retained) >= runOutputsMaxItems {
			truncated = true
			break
		}
		retained[key] = value
		total += entryBytes
	}
	return retained, truncated
}
