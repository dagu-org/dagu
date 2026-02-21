package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDAGWritesDisabledInReadOnlyMode(t *testing.T) {
	// Setup server with gitSync.enabled=true, pushEnabled=false (read-only mode)
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.GitSync.Enabled = true
		cfg.GitSync.PushEnabled = false
	}))

	t.Run("CreateNewDAG", func(t *testing.T) {
		spec := "steps:\n  - command: echo test"
		server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: "test_dag",
			Spec: &spec,
		}).ExpectStatus(http.StatusForbidden).Send(t)
	})

	t.Run("DeleteDAG", func(t *testing.T) {
		server.Client().Delete("/api/v1/dags/any_dag").
			ExpectStatus(http.StatusForbidden).Send(t)
	})

	t.Run("UpdateDAGSpec", func(t *testing.T) {
		server.Client().Put("/api/v1/dags/any_dag/spec", api.UpdateDAGSpecJSONRequestBody{
			Spec: "steps:\n  - command: echo updated",
		}).ExpectStatus(http.StatusForbidden).Send(t)
	})

	t.Run("RenameDAG", func(t *testing.T) {
		server.Client().Post("/api/v1/dags/any_dag/rename", api.RenameDAGJSONRequestBody{
			NewFileName: "new_name",
		}).ExpectStatus(http.StatusForbidden).Send(t)
	})
}

func TestDAGWritesAllowedWhenPushEnabled(t *testing.T) {
	// Setup server with gitSync.enabled=true, pushEnabled=true
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.GitSync.Enabled = true
		cfg.GitSync.PushEnabled = true
	}))

	// Test CreateNewDAG is allowed
	spec := "steps:\n  - command: echo test"
	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "test_dag_push_enabled",
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Cleanup
	server.Client().Delete("/api/v1/dags/test_dag_push_enabled").ExpectStatus(http.StatusNoContent).Send(t)
}

func TestDAGWritesAllowedWhenGitSyncDisabled(t *testing.T) {
	// Setup server with gitSync.enabled=false (default)
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.GitSync.Enabled = false
	}))

	// Test CreateNewDAG is allowed
	spec := "steps:\n  - command: echo test"
	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "test_dag_gitsync_disabled",
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Cleanup
	server.Client().Delete("/api/v1/dags/test_dag_gitsync_disabled").ExpectStatus(http.StatusNoContent).Send(t)
}

func TestCreateNewDAGPathTraversal(t *testing.T) {
	server := test.SetupServer(t)

	traversalNames := []string{
		"../../tmp/traversal",
		"../escape",
		"foo/bar",
		"../../../etc/malicious",
	}

	for _, name := range traversalNames {
		t.Run("with_spec/"+name, func(t *testing.T) {
			spec := "steps:\n  - command: echo test"
			server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
				Name: name,
				Spec: &spec,
			}).ExpectStatus(http.StatusBadRequest).Send(t)
		})

		t.Run("without_spec/"+name, func(t *testing.T) {
			server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
				Name: name,
			}).ExpectStatus(http.StatusBadRequest).Send(t)
		})
	}

	t.Run("empty_name", func(t *testing.T) {
		server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: "",
		}).ExpectStatus(http.StatusBadRequest).Send(t)
	})

	t.Run("dot_dot_name", func(t *testing.T) {
		server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: "..",
		}).ExpectStatus(http.StatusBadRequest).Send(t)
	})
}

func TestDAG(t *testing.T) {
	server := test.SetupServer(t)

	t.Run("CreateExecuteDelete", func(t *testing.T) {
		spec := `
steps:
  - sleep 1
`
		// Create a new DAG
		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: "test_dag",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Fetch the created DAG with the list endpoint
		resp := server.Client().Get("/api/v1/dags?name=test_dag").ExpectStatus(http.StatusOK).Send(t)
		var apiResp api.ListDAGs200JSONResponse
		resp.Unmarshal(t, &apiResp)

		require.Len(t, apiResp.Dags, 1, "expected one DAG")

		// Execute the created DAG
		resp = server.Client().Post("/api/v1/dags/test_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)

		require.NotEmpty(t, execResp.DagRunId, "expected a non-empty dag-run ID")

		// Check the status of the dag-run
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v1/dags/test_dag/dag-runs/%s", execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)

			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
		}, 5*time.Second, 1*time.Second, "expected DAG to complete")

		// Delete the created DAG
		_ = server.Client().Delete("/api/v1/dags/test_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ListDAGsSorting", func(t *testing.T) {
		// Test that ListDAGs respects sort parameters
		resp := server.Client().Get("/api/v1/dags?sort=name&order=asc").ExpectStatus(http.StatusOK).Send(t)
		var apiResp api.ListDAGs200JSONResponse
		resp.Unmarshal(t, &apiResp)

		// The test should pass regardless of the sort result
		// since we're only testing that the endpoint accepts the parameters
		require.NotNil(t, apiResp.Dags)
		require.NotNil(t, apiResp.Pagination)
	})

	t.Run("ExecuteDAGWithSingleton", func(t *testing.T) {
		// Create a new DAG
		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAG201JSONResponse{
			Name: "test_singleton_dag",
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Execute the DAG with singleton flag
		singleton := true
		resp := server.Client().Post("/api/v1/dags/test_singleton_dag/start", api.ExecuteDAGJSONRequestBody{
			Singleton: &singleton,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId, "expected a non-empty dag-run ID")

		// Clean up
		_ = server.Client().Delete("/api/v1/dags/test_singleton_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ExecuteDAGWithJSONParams", func(t *testing.T) {
		// Case 1: DAG with named params defined - JSON keys map to those params.
		// Verifies that JSON parameters are parsed as named key-value pairs,
		// not tokenized by whitespace (regression test for JSON params bug).
		spec := `
params:
  - name: key1
    default: default1
  - name: key2
    default: default2
steps:
  - name: echo_params
    command: echo "key1=$key1 key2=$key2"
`
		dagName := "test_json_params"

		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		jsonParams := `{"key1": "test1", "key2": "test2"}`
		resp := server.Client().Post("/api/v1/dags/"+dagName+"/start", api.ExecuteDAGJSONRequestBody{
			Params: &jsonParams,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId)

		var dagRunDetails api.GetDAGDAGRunDetails200JSONResponse
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			statusResp.Unmarshal(t, &dagRunDetails)
			return dagRunDetails.DagRun.Status == api.Status(core.Succeeded)
		}, 10*time.Second, 500*time.Millisecond, "DAG should complete")

		require.NotNil(t, dagRunDetails.DagRun.Params)
		params := *dagRunDetails.DagRun.Params
		require.Contains(t, params, "key1=test1")
		require.Contains(t, params, "key2=test2")
		require.NotContains(t, params, "1={", "JSON should not be tokenized")

		_ = server.Client().Delete("/api/v1/dags/" + dagName).ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ExecuteDAGWithJSONPositionalArg", func(t *testing.T) {
		// Case 2: No named params defined - JSON passed as positional arg.
		// The entire JSON is stored as $1 and accessible via JSON path syntax ${1.key}.
		spec := `
steps:
  - name: show_json
    command: echo "key1=${1.key1} key2=${1.key2}"
`
		dagName := "test_json_positional"

		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Pass JSON as a single positional argument using array syntax
		jsonParams := `["{\"key1\": \"val1\", \"key2\": \"val2\"}"]`
		resp := server.Client().Post("/api/v1/dags/"+dagName+"/start", api.ExecuteDAGJSONRequestBody{
			Params: &jsonParams,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId)

		var dagRunDetails api.GetDAGDAGRunDetails200JSONResponse
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			statusResp.Unmarshal(t, &dagRunDetails)
			return dagRunDetails.DagRun.Status == api.Status(core.Succeeded)
		}, 10*time.Second, 500*time.Millisecond, "DAG should complete")

		require.NotNil(t, dagRunDetails.DagRun.Params)
		params := *dagRunDetails.DagRun.Params
		// Positional arg $1 should contain the full JSON string
		require.Contains(t, params, `1={"key1": "val1", "key2": "val2"}`)

		_ = server.Client().Delete("/api/v1/dags/" + dagName).ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ExecuteDAGAllowsFewerPositionalParamsThanDeclared", func(t *testing.T) {
		spec := `
params: "p1 p2"
steps:
  - name: echo
    command: echo "$1 $2"
`
		dagName := "test_positional_fewer_allowed"

		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		params := "one"
		resp := server.Client().Post("/api/v1/dags/"+dagName+"/start", api.ExecuteDAGJSONRequestBody{
			Params: &params,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId)
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
		}, 5*time.Second, 500*time.Millisecond)

		_ = server.Client().Delete("/api/v1/dags/" + dagName).ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ExecuteDAGRejectsTooManyPositionalParams", func(t *testing.T) {
		spec := `
params: "p1 p2"
steps:
  - name: echo
    command: echo "$1 $2"
`
		dagName := "test_positional_too_many_rejected"

		_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		params := "one two three"
		resp := server.Client().Post("/api/v1/dags/"+dagName+"/start", api.ExecuteDAGJSONRequestBody{
			Params: &params,
		}).ExpectStatus(http.StatusBadRequest).Send(t)

		var errResp api.Error
		resp.Unmarshal(t, &errResp)
		require.Equal(t, api.ErrorCodeBadRequest, errResp.Code)
		require.Contains(t, errResp.Message, "too many positional params: expected at most 2, got 3")

		_ = server.Client().Delete("/api/v1/dags/" + dagName).ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("EnqueueDAGRunFromSpec", func(t *testing.T) {
		spec := `
steps:
  - sleep 1
`
		name := "inline_enqueue_spec"

		resp := server.Client().Post("/api/v1/dag-runs/enqueue", api.EnqueueDAGRunFromSpecJSONRequestBody{
			Spec: spec,
			Name: &name,
		}).
			ExpectStatus(http.StatusOK).
			Send(t)

		var body api.EnqueueDAGRunFromSpec200JSONResponse
		resp.Unmarshal(t, &body)
		require.NotEmpty(t, body.DagRunId, "expected a non-empty dag-run ID")

		require.Eventually(t, func() bool {
			statusResp := server.Client().
				Get(fmt.Sprintf("/api/v1/dag-runs/%s/%s", name, body.DagRunId)).
				ExpectStatus(http.StatusOK).
				Send(t)

			var dagRun api.GetDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRun)

			s := dagRun.DagRunDetails.Status
			return s == api.Status(core.Queued) || s == api.Status(core.Running) || s == api.Status(core.Succeeded)
		}, 5*time.Second, 250*time.Millisecond, "expected DAG-run to reach queued state")
	})
}
