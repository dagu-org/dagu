package integration

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
)

func TestIntegration(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		dag             string
		expectedOutputs map[string]any
	}

	testCases := []testCase{
		{
			name: "Depends",
			dag:  "depends.yaml",
		},
		{
			name: "Pipe",
			dag:  "pipe.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "hello foo",
			},
		},
		{
			name: "NamedParams",
			dag:  "named-params.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Dagu",
				"OUT2": "Hello, Dagu",
			},
		},
		{
			name: "NamedParamsList",
			dag:  "named-params-list.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Dagu",
				"OUT2": "Hello, Dagu",
			},
		},
		{
			name: "PositionalParams",
			dag:  "positional-params.yaml",
			expectedOutputs: map[string]any{
				"OUT1": []test.Contains{
					"$1 is foo",
					"$2 is bar",
				},
			},
		},
		{
			name: "Script",
			dag:  "script.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "1 2 3",
			},
		},
		{
			name: "RegexPrecondition",
			dag:  "precondition-regex.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "abc run def",
				"OUT2": "match",
			},
		},
		{
			name: "Json",
			dag:  "json.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "Starting server at localhost:8080",
			},
		},
		{
			name: "EnvironmentVar",
			dag:  "environment-var.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "foo",
			},
		},
		{
			name: "SpecialVars",
			dag:  "special-vars.yaml",
			expectedOutputs: map[string]any{
				"OUT1": test.NotEmpty{},
				"OUT2": test.NotEmpty{},
				"OUT3": test.NotEmpty{},
				"OUT4": test.NotEmpty{},
				"OUT5": test.NotEmpty{},
			},
		},
		{
			name: "Issue-810",
			dag:  "issue-810.yaml",
			expectedOutputs: map[string]any{
				"OUT1": "start",
				"OUT2": "foo",
				"OUT3": "bar",
				"OUT4": "baz",
			},
		},
	}

	th := test.Setup(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dag := th.DAG(t, filepath.Join("integration", tc.dag))
			agent := dag.Agent()

			agent.RunSuccess(t)

			dag.AssertLatestStatus(t, scheduler.StatusSuccess)
			dag.AssertOutputs(t, tc.expectedOutputs)
		})
	}
}
