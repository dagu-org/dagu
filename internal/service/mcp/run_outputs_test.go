// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"

	daguapi "github.com/dagucloud/dagu/v2/api/v1"
	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/persis"
	persisfile "github.com/dagucloud/dagu/v2/internal/persis/file"
	filedag "github.com/dagucloud/dagu/v2/internal/persis/file/dag"
	persiststore "github.com/dagucloud/dagu/v2/internal/persis/store"
	"github.com/dagucloud/dagu/v2/internal/runtime"
	frontendapi "github.com/dagucloud/dagu/v2/internal/service/frontend/api/v1"
	"github.com/dagucloud/dagu/v2/internal/spec"
	"github.com/dagucloud/dagu/v2/internal/testutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func twoDigits(i int) string {
	return fmt.Sprintf("%02d", i)
}

// A JSON string result stays verbatim so assessment clients can parse it.
func TestRunOutputsEnvelopeRetainsJSONStringResult(t *testing.T) {
	t.Parallel()

	report := `{"ok":true,"rows":3}`
	envelope := runOutputsEnvelope(&ir.DAGRunOutputs{
		Outputs: map[string]string{"report": report, "summary": "plain"},
	}, nil)

	require.Equal(t, "ok", envelope["outputsStatus"])
	require.NotContains(t, envelope, "outputsTruncated")
	require.NotContains(t, envelope, "outputsError")
	outputs, ok := envelope["outputs"].(map[string]string)
	require.True(t, ok)
	require.Equal(t, map[string]string{"report": report, "summary": "plain"}, outputs)
}

func TestRunOutputsEnvelopeEmptyAndUnavailable(t *testing.T) {
	t.Parallel()

	empty := runOutputsEnvelope(&ir.DAGRunOutputs{Outputs: map[string]string{}}, nil)
	require.Equal(t, "empty", empty["outputsStatus"])
	require.Empty(t, empty["outputs"])

	// No outputs file yet, as on an active run, is unavailable rather than empty.
	unavailable := runOutputsEnvelope(nil, nil)
	require.Equal(t, "unavailable", unavailable["outputsStatus"])
	require.Empty(t, unavailable["outputs"])
}

// An outputs failure is recorded without touching the terminal state the read
// already resolved, including a permission failure mapped to not found.
func TestRunOutputsEnvelopeErrorKeepsTerminalState(t *testing.T) {
	t.Parallel()

	for _, readErr := range []error{
		errors.New("error reading outputs: boom"),
		errors.New("dag-run ID run-1 not found for DAG etl"),
	} {
		data := map[string]any{"status": 2, "statusLabel": "failed"}
		maps.Copy(data, runOutputsEnvelope(nil, readErr))
		require.Equal(t, 2, data["status"])
		require.Equal(t, "failed", data["statusLabel"])
		require.Equal(t, "error", data["outputsStatus"])
		require.Equal(t, readErr.Error(), data["outputsError"])
		require.Empty(t, data["outputs"])
	}
}

func TestBoundedRunOutputsEnforcesItemAndByteBudgets(t *testing.T) {
	t.Parallel()

	many := make(map[string]string, runOutputsMaxItems+10)
	for i := range runOutputsMaxItems + 10 {
		many["output"+twoDigits(i)] = strings.Repeat("x", 8)
	}
	retained, truncated := boundedRunOutputs(many)
	require.True(t, truncated)
	require.Len(t, retained, runOutputsMaxItems)

	// An oversized entry is omitted intact, never cut, while later small
	// entries are still considered.
	mixed := map[string]string{
		"aaa_big": strings.Repeat("a", runOutputsMaxValueBytes),
		"zzz_ok":  `{"ok":true}`,
	}
	retained, truncated = boundedRunOutputs(mixed)
	require.True(t, truncated)
	require.Equal(t, map[string]string{"zzz_ok": `{"ok":true}`}, retained)

	// Entries that would overflow the total budget are skipped while a later
	// small entry is still retained.
	full := map[string]string{"k6": "small"}
	for _, key := range []string{"k1", "k2", "k3", "k4", "k5"} {
		full[key] = strings.Repeat("x", 15*1024)
	}
	retained, truncated = boundedRunOutputs(full)
	require.True(t, truncated)
	require.Len(t, retained, 5)
	require.Equal(t, "small", retained["k6"])
	require.NotContains(t, retained, "k5")
}

// outputsFixture serves the Dagu API over file stores holding one succeeded
// run with a JSON string result plus a child run with its own outputs.
func outputsFixture(t *testing.T) *Service {
	t.Helper()

	const report = `{"ok":true,"rows":3}`
	ctx := context.Background()
	dataDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Paths.DataDir = dataDir
	cfg.Paths.DAGRunsDir = filepath.Join(dataDir, "dag-runs")
	cfg.Paths.ProcDir = filepath.Join(dataDir, "proc")
	cfg.Paths.QueueDir = filepath.Join(dataDir, "queue")

	runs := persisfile.NewDAGRunRepository(cfg)
	procs := persisfile.NewProcRepository(cfg)
	queueStore := persiststore.NewQueueStore(persisfile.NewBackend(cfg.Paths).Collection(persis.CollectionQueue))
	dags := testutil.NewFileDAGRepository(t.TempDir(), filedag.WithSkipExamples(true))
	api := frontendapi.New(dags, runs, queueStore, procs, runtime.NewManager(runs, procs, cfg), cfg, nil, nil, prometheus.NewRegistry(), nil)

	writeRun := func(dag *ir.DAG, dagRunID string, opts persis.DAGRunCreateAttemptOptions, outputs map[string]string) {
		t.Helper()
		attempt, err := runs.CreateAttempt(ctx, dag, time.Now(), dagRunID, opts)
		require.NoError(t, err)
		require.NoError(t, attempt.Open(ctx))
		status := ir.InitialStatus(dag)
		status.DAGRunID = dagRunID
		status.AttemptID = attempt.ID()
		status.Status = ir.Succeeded
		for _, node := range status.Nodes {
			node.Status = ir.NodeSucceeded
		}
		require.NoError(t, attempt.Write(ctx, status))
		require.NoError(t, attempt.WriteOutputs(ctx, &ir.DAGRunOutputs{
			Metadata: ir.OutputsMetadata{DAGName: dag.Name, DAGRunID: dagRunID, AttemptID: attempt.ID(), Status: "succeeded"},
			Outputs:  outputs,
		}))
		require.NoError(t, attempt.Close(ctx))
	}

	dag, err := spec.LoadYAML(ctx, []byte("name: etl\nsteps:\n  - name: extract\n    run: \"true\"\n"))
	require.NoError(t, err)
	writeRun(dag, "run-1", persis.DAGRunCreateAttemptOptions{}, map[string]string{"report": report})

	childDAG, err := spec.LoadYAML(ctx, []byte("name: child\nsteps:\n  - name: main\n    run: \"true\"\n"))
	require.NoError(t, err)
	writeRun(childDAG, "child-run", persis.DAGRunCreateAttemptOptions{RootDAGRun: ir.NewDAGRunRef("etl", "run-1")}, map[string]string{"child_report": report})

	return &Service{api: api}
}

func TestMCPReadRunIncludesBoundedOutputs(t *testing.T) {
	t.Parallel()

	svc := outputsFixture(t)
	ctx := context.Background()

	_, output, err := svc.readToolImpl(ctx, readInput{Target: readTargetRun, Name: "etl", DAGRunID: "run-1"})
	require.NoError(t, err)
	data, ok := output["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", data["outputsStatus"])
	require.Equal(t, map[string]string{"report": `{"ok":true,"rows":3}`}, data["outputs"])

	_, output, err = svc.readToolImpl(ctx, readInput{Target: readTargetRun, Name: "etl", DAGRunID: "run-1", SubRunID: "child-run"})
	require.NoError(t, err)
	data, ok = output["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", data["outputsStatus"])
	require.Equal(t, map[string]string{"child_report": `{"ok":true,"rows":3}`}, data["outputs"])
}

func TestMCPWaitForRunIncludesBoundedOutputs(t *testing.T) {
	t.Parallel()

	svc := outputsFixture(t)
	output := map[string]any{}
	message := svc.waitForRun(context.Background(),
		executeInput{Action: executeActionStart, Name: "etl", Wait: true, WaitTimeoutSeconds: 5}, "run-1", output)
	require.Equal(t, true, output["completed"])
	require.Equal(t, 4, int(output["status"].(daguapi.Status)))
	run, ok := output["run"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", run["outputsStatus"])
	require.Equal(t, map[string]string{"report": `{"ok":true,"rows":3}`}, run["outputs"])
	require.Contains(t, message, "succeeded")
}

func TestRunOutputsStayWithResolvedRunAndAttempt(t *testing.T) {
	t.Parallel()
	svc := outputsFixture(t)
	ctx := context.Background()
	raw, err := svc.api.GetDAGRunDetailsData(ctx, "etl/latest")
	require.NoError(t, err)
	details, err := normalizeRunDetails(raw, runAddress{name: "etl", dagRunID: "latest"})
	require.NoError(t, err)
	svc.attachBoundedRunOutputs(ctx, details, runAddress{name: "etl", dagRunID: "no-longer-resolves"})
	require.Equal(t, "ok", details["outputsStatus"])
	require.Equal(t, map[string]string{"report": `{"ok":true,"rows":3}`}, details["outputs"])
	details["attemptId"] = "stale-attempt"
	svc.attachBoundedRunOutputs(ctx, details, runAddress{name: "etl", dagRunID: "run-1"})
	require.Equal(t, "error", details["outputsStatus"])
	require.Empty(t, details["outputs"])
	require.Contains(t, details["outputsError"], "attempt changed")
	require.Equal(t, daguapi.Status(4), details["status"])
}

func TestMCPResourceRunReadIncludesBoundedOutputs(t *testing.T) {
	t.Parallel()

	svc := outputsFixture(t)
	ctx := context.Background()

	// A latest alias reads the run it identified, keeping the response shape.
	raw, err := svc.api.GetDAGRunDetailsData(ctx, "etl/latest")
	require.NoError(t, err)
	shaped := svc.withResourceRunOutputs(ctx, raw, runAddress{name: "etl", dagRunID: "latest"})
	data, ok := shaped.(map[string]any)
	require.True(t, ok)
	require.Contains(t, data, "dagRunDetails")
	require.Equal(t, "ok", data["outputsStatus"])
	require.Equal(t, map[string]string{"report": `{"ok":true,"rows":3}`}, data["outputs"])

	raw, err = svc.api.GetSubDAGRunDetailsData(ctx, "etl/run-1/child-run")
	require.NoError(t, err)
	shaped = svc.withResourceRunOutputs(ctx, raw, runAddress{name: "etl", dagRunID: "run-1", subRunID: "child-run"})
	data, ok = shaped.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", data["outputsStatus"])
	require.Equal(t, map[string]string{"child_report": `{"ok":true,"rows":3}`}, data["outputs"])
}

func TestBoundedRunOutputsTruncationIsDeterministic(t *testing.T) {
	t.Parallel()

	outputs := map[string]string{
		"b": strings.Repeat("b", 1024),
		"a": strings.Repeat("a", 1024),
	}
	first, _ := boundedRunOutputs(outputs)
	second, _ := boundedRunOutputs(outputs)
	require.Equal(t, first, second)
	require.Contains(t, first["a"], "a")
}
