package core_test

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSockAddr(t *testing.T) {
	t.Parallel()

	t.Run("Location", func(t *testing.T) {
		dag := &core.DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu_testdata_testDag_yml_[0-9a-f]+\.sock$`, dag.SockAddr(""))
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		dag := &core.DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 50 is an application-imposed limit to keep socket names short and portable
		// (the system limit UNIX_PATH_MAX is typically 108 bytes on Linux)
		require.LessOrEqual(t, 50, len(dag.SockAddr("")))
		require.Equal(
			t,
			"/tmp/@dagu_testdata_testDagVeryLongNameThat_b92b71.sock",
			dag.SockAddr(""),
		)
	})
	t.Run("BasicFunctionality", func(t *testing.T) {
		t.Parallel()

		addr := core.SockAddr("", "mydag", "run123")

		require.True(t, strings.HasPrefix(addr, "/tmp/@dagu_"))
		require.True(t, strings.HasSuffix(addr, ".sock"))
		require.Contains(t, addr, "mydag")

		// Verify 6-character hash
		parts := strings.Split(addr, "_")
		lastPart := parts[len(parts)-1]
		hashPart := strings.TrimSuffix(lastPart, ".sock")
		require.Len(t, hashPart, 6)

		// Deterministic: same inputs produce same output
		require.Equal(t, addr, core.SockAddr("", "mydag", "run123"))
	})

	t.Run("DifferentInputsProduceDifferentHashes", func(t *testing.T) {
		t.Parallel()

		addr1 := core.SockAddr("", "dag1", "run1")
		addr2 := core.SockAddr("", "dag1", "run2")
		addr3 := core.SockAddr("", "dag2", "run1")

		require.NotEqual(t, addr1, addr2, "different dagRunIDs should produce different addresses")
		require.NotEqual(t, addr1, addr3, "different names should produce different addresses")
	})

	t.Run("DifferentNamespacesProduceDifferentPaths", func(t *testing.T) {
		t.Parallel()

		addr1 := core.SockAddr("ns-alpha", "mydag", "run1")
		addr2 := core.SockAddr("ns-beta", "mydag", "run1")

		require.NotEqual(t, addr1, addr2, "different namespaces should produce different addresses for same DAG name")
	})

	t.Run("SafeNameHandling", func(t *testing.T) {
		t.Parallel()

		addr := core.SockAddr("", "my/dag\\with:special*chars", "run|with<>chars")
		socketName := strings.TrimPrefix(addr, "/tmp/")

		// Verify unsafe characters are sanitized
		for _, char := range []string{"/", "\\", ":", "*", "|", "<", ">"} {
			require.NotContains(t, socketName, char)
		}
	})

	t.Run("MaxSocketLengthEnforcement", func(t *testing.T) {
		t.Parallel()

		addr := core.SockAddr("", strings.Repeat("a", 100), "run123")
		socketName := strings.TrimPrefix(addr, "/tmp/")

		require.LessOrEqual(t, len(socketName), 50)
		require.True(t, strings.HasPrefix(socketName, "@dagu_"))
		require.True(t, strings.HasSuffix(socketName, ".sock"))
	})

	t.Run("EdgeCaseTruncation", func(t *testing.T) {
		t.Parallel()

		// Format: @dagu_ (6) + name (?) + _ (1) + hash (6) + .sock (5) = 50
		// Max name length = 50 - 6 - 1 - 6 - 5 = 32
		name32 := strings.Repeat("x", 32)
		name33 := strings.Repeat("x", 33)

		socketName32 := strings.TrimPrefix(core.SockAddr("", name32, "run123"), "/tmp/")
		socketName33 := strings.TrimPrefix(core.SockAddr("", name33, "run123"), "/tmp/")

		require.LessOrEqual(t, len(socketName32), 50)
		require.LessOrEqual(t, len(socketName33), 50)
		require.Contains(t, socketName32, name32)
		require.NotContains(t, socketName33, name33, "33-char name should be truncated")
	})

	t.Run("EmptyInputs", func(t *testing.T) {
		t.Parallel()

		addrs := []string{
			core.SockAddr("", "", ""),
			core.SockAddr("", "dag", ""),
			core.SockAddr("", "", "run"),
		}

		for _, addr := range addrs {
			require.True(t, strings.HasPrefix(addr, "/tmp/@dagu_"))
			require.True(t, strings.HasSuffix(addr, ".sock"))
		}
	})

	t.Run("HashConsistency", func(t *testing.T) {
		t.Parallel()

		ns, name, runID := "testns", "testdag", "testrun"
		addr := core.SockAddr(ns, name, runID)

		parts := strings.Split(addr, "_")
		hash := strings.TrimSuffix(parts[len(parts)-1], ".sock")
		expectedHash := fmt.Sprintf("%x", md5.Sum([]byte(ns+name+runID)))[:6]

		require.Equal(t, expectedHash, hash)
	})

	t.Run("DAGMethodsUseSockAddr", func(t *testing.T) {
		t.Parallel()

		// With Location set: uses Location and Namespace
		dag1 := &core.DAG{Name: "mydag", Location: "path/to/dag.yml", Namespace: "ns1"}
		require.Equal(t, core.SockAddr("ns1", "path/to/dag.yml", ""), dag1.SockAddr("run123"))

		// Without Location: uses Name, Namespace, and dagRunID
		dag2 := &core.DAG{Name: "mydag", Namespace: "ns1"}
		require.Equal(t, core.SockAddr("ns1", "mydag", "run123"), dag2.SockAddr("run123"))
	})

	t.Run("SockAddrForSubDAGRun", func(t *testing.T) {
		t.Parallel()

		dag := &core.DAG{Name: "parentdag", Location: "path/to/parent.yml", Namespace: "ns1"}
		subRunID := "child-run-456"

		// Uses DAG name (not location) with the sub run ID and Namespace
		require.Equal(t, core.SockAddr("ns1", "parentdag", subRunID), dag.SockAddrForSubDAGRun(subRunID))
	})
}

func TestMarshalJSON(t *testing.T) {
	th := test.Setup(t)
	t.Run("MarshalJSON", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)
		_, err := json.Marshal(dag.DAG)
		require.NoError(t, err)
	})
}

func TestScheduleJSON(t *testing.T) {
	t.Parallel()

	t.Run("MarshalUnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		original := core.Schedule{Expression: "0 0 * * *"}
		parsed, err := cron.ParseStandard(original.Expression)
		require.NoError(t, err)
		original.Parsed = parsed

		data, err := json.Marshal(original)
		require.NoError(t, err)

		jsonStr := string(data)
		require.Contains(t, jsonStr, `"expression":"0 0 * * *"`)
		require.NotContains(t, jsonStr, `"Expression"`)
		require.NotContains(t, jsonStr, `"Parsed"`)

		var unmarshaled core.Schedule
		require.NoError(t, json.Unmarshal(data, &unmarshaled))
		require.Equal(t, original.Expression, unmarshaled.Expression)
		require.NotNil(t, unmarshaled.Parsed)

		now := time.Now()
		require.Equal(t, original.Parsed.Next(now), unmarshaled.Parsed.Next(now))
	})

	t.Run("UnmarshalInvalidCron", func(t *testing.T) {
		t.Parallel()

		var schedule core.Schedule
		err := json.Unmarshal([]byte(`{"expression":"invalid cron"}`), &schedule)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron expression")
	})
}

func TestNextRun(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Schedule: []core.Schedule{{Expression: "0 1 * * *"}},
	}
	parsed, err := cron.ParseStandard(dag.Schedule[0].Expression)
	require.NoError(t, err)
	dag.Schedule[0].Parsed = parsed

	now := time.Date(2023, 10, 1, 1, 0, 0, 0, time.UTC)
	expected := time.Date(2023, 10, 2, 1, 0, 0, 0, time.UTC)

	require.Equal(t, expected, dag.NextRun(now))
}

func TestEffectiveLogOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		dagLogOutput  core.LogOutputMode
		stepLogOutput core.LogOutputMode
		expected      core.LogOutputMode
	}{
		{
			name:          "BothEmpty_ReturnsSeparate",
			dagLogOutput:  "",
			stepLogOutput: "",
			expected:      core.LogOutputSeparate,
		},
		{
			name:          "DAGSeparate_StepEmpty_ReturnsSeparate",
			dagLogOutput:  core.LogOutputSeparate,
			stepLogOutput: "",
			expected:      core.LogOutputSeparate,
		},
		{
			name:          "DAGMerged_StepEmpty_ReturnsMerged",
			dagLogOutput:  core.LogOutputMerged,
			stepLogOutput: "",
			expected:      core.LogOutputMerged,
		},
		{
			name:          "DAGEmpty_StepSeparate_ReturnsSeparate",
			dagLogOutput:  "",
			stepLogOutput: core.LogOutputSeparate,
			expected:      core.LogOutputSeparate,
		},
		{
			name:          "DAGEmpty_StepMerged_ReturnsMerged",
			dagLogOutput:  "",
			stepLogOutput: core.LogOutputMerged,
			expected:      core.LogOutputMerged,
		},
		{
			name:          "DAGSeparate_StepMerged_StepOverrides",
			dagLogOutput:  core.LogOutputSeparate,
			stepLogOutput: core.LogOutputMerged,
			expected:      core.LogOutputMerged,
		},
		{
			name:          "DAGMerged_StepSeparate_StepOverrides",
			dagLogOutput:  core.LogOutputMerged,
			stepLogOutput: core.LogOutputSeparate,
			expected:      core.LogOutputSeparate,
		},
		{
			name:          "NilDAG_StepMerged_ReturnsMerged",
			dagLogOutput:  "", // Will use nil DAG
			stepLogOutput: core.LogOutputMerged,
			expected:      core.LogOutputMerged,
		},
		{
			name:          "NilStep_DAGMerged_ReturnsMerged",
			dagLogOutput:  core.LogOutputMerged,
			stepLogOutput: "", // Will use nil Step
			expected:      core.LogOutputMerged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var dag *core.DAG
			var step *core.Step

			// Setup DAG
			if tt.name != "NilDAG_StepMerged_ReturnsMerged" {
				dag = &core.DAG{LogOutput: tt.dagLogOutput}
			}

			// Setup Step
			if tt.name != "NilStep_DAGMerged_ReturnsMerged" {
				step = &core.Step{LogOutput: tt.stepLogOutput}
			}

			result := core.EffectiveLogOutput(dag, step)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDAG_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		dag    *core.DAG
		errMsg string
	}{
		{
			name: "valid DAG with name passes",
			dag: &core.DAG{
				Name:  "test-dag",
				Steps: []core.Step{{Name: "step1"}},
			},
		},
		{
			name:   "empty name fails",
			dag:    &core.DAG{Name: ""},
			errMsg: "DAG name is required",
		},
		{
			name: "valid dependencies pass",
			dag: &core.DAG{
				Name: "test-dag",
				Steps: []core.Step{
					{Name: "step1"},
					{Name: "step2", Depends: []string{"step1"}},
				},
			},
		},
		{
			name: "missing dependency fails",
			dag: &core.DAG{
				Name: "test-dag",
				Steps: []core.Step{
					{Name: "step1"},
					{Name: "step2", Depends: []string{"nonexistent"}},
				},
			},
			errMsg: "non-existent step",
		},
		{
			name: "complex multi-level dependencies pass",
			dag: &core.DAG{
				Name: "test-dag",
				Steps: []core.Step{
					{Name: "step1"},
					{Name: "step2", Depends: []string{"step1"}},
					{Name: "step3", Depends: []string{"step1", "step2"}},
					{Name: "step4", Depends: []string{"step3"}},
				},
			},
		},
		{
			name: "steps with no dependencies pass",
			dag: &core.DAG{
				Name: "test-dag",
				Steps: []core.Step{
					{Name: "step1"},
					{Name: "step2"},
					{Name: "step3"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.dag.Validate()
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDAG_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name: "",
		Steps: []core.Step{
			{Name: "a", Depends: []string{"missing1"}},
			{Name: "b", Depends: []string{"missing2"}},
			{Name: "c", Depends: []string{"missing3"}},
		},
	}

	err := dag.Validate()
	require.Error(t, err)

	var errList core.ErrorList
	require.True(t, errors.As(err, &errList), "error should be an ErrorList")
	assert.Len(t, errList, 4, "should collect all 4 errors (1 name + 3 dependencies)")

	errStr := err.Error()
	for _, expected := range []string{"DAG name is required", "missing1", "missing2", "missing3"} {
		assert.Contains(t, errStr, expected)
	}
}

func TestDAG_HasTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tags     []string
		search   string
		expected bool
	}{
		{
			name:     "empty tags, search for any returns false",
			tags:     []string{},
			search:   "test",
			expected: false,
		},
		{
			name:     "has tag, search for it returns true",
			tags:     []string{"production", "critical"},
			search:   "production",
			expected: true,
		},
		{
			name:     "has tag, search for different returns false",
			tags:     []string{"production", "critical"},
			search:   "staging",
			expected: false,
		},
		{
			name:     "multiple tags, search for last one returns true",
			tags:     []string{"a", "b", "c", "d"},
			search:   "d",
			expected: true,
		},
		{
			name:     "case insensitive - uppercase search matches lowercase tag",
			tags:     []string{"production"},
			search:   "PRODUCTION",
			expected: true,
		},
		{
			name:     "case insensitive - lowercase search matches uppercase tag",
			tags:     []string{"Production"},
			search:   "production",
			expected: true,
		},
		{
			name:     "nil tags returns false",
			tags:     nil,
			search:   "test",
			expected: false,
		},
		{
			name:     "key-value tag with exact match",
			tags:     []string{"env=prod"},
			search:   "env=prod",
			expected: true,
		},
		{
			name:     "key-value tag with key-only search",
			tags:     []string{"env=prod"},
			search:   "env",
			expected: true,
		},
		{
			name:     "key-value tag with wrong value",
			tags:     []string{"env=prod"},
			search:   "env=staging",
			expected: false,
		},
		{
			name:     "negation filter - key not present",
			tags:     []string{"env=prod"},
			search:   "!deprecated",
			expected: true,
		},
		{
			name:     "negation filter - key present",
			tags:     []string{"env=prod", "deprecated"},
			search:   "!deprecated",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{Tags: core.NewTags(tt.tags)}
			result := dag.HasTag(tt.search)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDAG_ParamsMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   []string
		expected map[string]string
	}{
		{
			name:     "empty params returns empty map",
			params:   []string{},
			expected: map[string]string{},
		},
		{
			name:     "single param key=value",
			params:   []string{"key=value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "multiple params",
			params:   []string{"key1=value1", "key2=value2", "key3=value3"},
			expected: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
		},
		{
			name:     "param with multiple equals - first splits",
			params:   []string{"key=value=with=equals"},
			expected: map[string]string{"key": "value=with=equals"},
		},
		{
			name:     "param without equals - excluded",
			params:   []string{"noequals"},
			expected: map[string]string{},
		},
		{
			name:     "mixed valid and invalid params",
			params:   []string{"valid=value", "invalid", "another=one"},
			expected: map[string]string{"valid": "value", "another": "one"},
		},
		{
			name:     "empty value",
			params:   []string{"key="},
			expected: map[string]string{"key": ""},
		},
		{
			name:     "nil params returns empty map",
			params:   nil,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{Params: tt.params}
			result := dag.ParamsMap()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDAG_ProcGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		queue    string
		dagName  string
		expected string
	}{
		{
			name:     "queue set returns queue",
			queue:    "my-queue",
			dagName:  "my-dag",
			expected: "my-queue",
		},
		{
			name:     "queue empty returns dag name",
			queue:    "",
			dagName:  "my-dag",
			expected: "my-dag",
		},
		{
			name:     "both empty returns empty string",
			queue:    "",
			dagName:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{Queue: tt.queue, Name: tt.dagName}
			result := dag.ProcGroup()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDAG_FileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		location string
		expected string
	}{
		{
			name:     "location with .yaml extension",
			location: "/path/to/mydag.yaml",
			expected: "mydag",
		},
		{
			name:     "location with .yml extension",
			location: "/path/to/mydag.yml",
			expected: "mydag",
		},
		{
			name:     "location with no extension",
			location: "/path/to/mydag",
			expected: "mydag",
		},
		{
			name:     "empty location returns empty string",
			location: "",
			expected: "",
		},
		{
			name:     "just filename with yaml",
			location: "simple.yaml",
			expected: "simple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{Location: tt.location}
			result := dag.FileName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDAG_String(t *testing.T) {
	t.Parallel()

	t.Run("full DAG formatted output", func(t *testing.T) {
		t.Parallel()

		dag := &core.DAG{
			Name:        "test-dag",
			Description: "A test DAG",
			Params:      []string{"param1=value1", "param2=value2"},
			LogDir:      "/var/log/dags",
			Steps:       []core.Step{{Name: "step1"}, {Name: "step2"}},
		}
		result := dag.String()

		for _, expected := range []string{"test-dag", "A test DAG", "param1=value1", "/var/log/dags"} {
			assert.Contains(t, result, expected)
		}
	})

	t.Run("minimal DAG basic output", func(t *testing.T) {
		t.Parallel()

		result := (&core.DAG{Name: "minimal"}).String()

		for _, expected := range []string{"minimal", "{", "}"} {
			assert.Contains(t, result, expected)
		}
	})
}

func TestDAG_InitializeDefaults(t *testing.T) {
	t.Parallel()

	t.Run("empty DAG sets all defaults", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{}
		core.InitializeDefaults(dag)

		assert.Equal(t, core.TypeChain, dag.Type)
		assert.Equal(t, 30, dag.HistRetentionDays)
		assert.Equal(t, 5*time.Second, dag.MaxCleanUpTime)
		assert.Equal(t, 1, dag.MaxActiveRuns)
		assert.Equal(t, 1024*1024, dag.MaxOutputSize)
	})

	t.Run("pre-existing Type not overwritten", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{Type: core.TypeGraph}
		core.InitializeDefaults(dag)

		assert.Equal(t, core.TypeGraph, dag.Type)
	})

	t.Run("pre-existing HistRetentionDays not overwritten", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{HistRetentionDays: 90}
		core.InitializeDefaults(dag)

		assert.Equal(t, 90, dag.HistRetentionDays)
	})

	t.Run("pre-existing MaxActiveRuns not overwritten", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{MaxActiveRuns: 5}
		core.InitializeDefaults(dag)

		assert.Equal(t, 5, dag.MaxActiveRuns)
	})

	t.Run("negative MaxActiveRuns preserved (deprecated)", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{MaxActiveRuns: -1}
		core.InitializeDefaults(dag)

		// Negative values are deprecated but still preserved for backwards compatibility
		// A build warning will be emitted when the DAG is loaded
		assert.Equal(t, -1, dag.MaxActiveRuns)
	})
}

func TestDAG_NextRun_Extended(t *testing.T) {
	t.Parallel()

	t.Run("empty schedule returns zero time", func(t *testing.T) {
		t.Parallel()

		dag := &core.DAG{Schedule: []core.Schedule{}}
		assert.True(t, dag.NextRun(time.Now()).IsZero())
	})

	t.Run("single schedule returns correct next time", func(t *testing.T) {
		t.Parallel()

		parsed, err := cron.ParseStandard("0 * * * *")
		require.NoError(t, err)

		dag := &core.DAG{
			Schedule: []core.Schedule{{Expression: "0 * * * *", Parsed: parsed}},
		}

		now := time.Date(2023, 10, 1, 12, 30, 0, 0, time.UTC)
		expected := time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC)

		assert.Equal(t, expected, dag.NextRun(now))
	})

	t.Run("multiple schedules returns earliest", func(t *testing.T) {
		t.Parallel()

		hourly, err := cron.ParseStandard("0 * * * *")
		require.NoError(t, err)
		halfHourly, err := cron.ParseStandard("*/30 * * * *")
		require.NoError(t, err)

		dag := &core.DAG{
			Schedule: []core.Schedule{
				{Expression: "0 * * * *", Parsed: hourly},
				{Expression: "*/30 * * * *", Parsed: halfHourly},
			},
		}

		now := time.Date(2023, 10, 1, 12, 15, 0, 0, time.UTC)
		expected := time.Date(2023, 10, 1, 12, 30, 0, 0, time.UTC)

		assert.Equal(t, expected, dag.NextRun(now))
	})

	t.Run("nil Parsed in schedule is skipped", func(t *testing.T) {
		t.Parallel()

		parsed, err := cron.ParseStandard("0 * * * *")
		require.NoError(t, err)

		dag := &core.DAG{
			Schedule: []core.Schedule{
				{Expression: "invalid", Parsed: nil},
				{Expression: "0 * * * *", Parsed: parsed},
			},
		}

		now := time.Date(2023, 10, 1, 12, 30, 0, 0, time.UTC)
		expected := time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC)

		assert.Equal(t, expected, dag.NextRun(now))
	})
}

func TestDAG_GetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dag      *core.DAG
		expected string
	}{
		{
			name:     "name set returns name",
			dag:      &core.DAG{Name: "my-dag", Location: "/path/to/other.yaml"},
			expected: "my-dag",
		},
		{
			name:     "name empty returns filename from location",
			dag:      &core.DAG{Name: "", Location: "/path/to/mydag.yaml"},
			expected: "mydag",
		},
		{
			name:     "name empty and location empty returns empty",
			dag:      &core.DAG{Name: "", Location: ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.dag.GetName())
		})
	}
}

func TestDAGHasHITLSteps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		steps    []core.Step
		expected bool
	}{
		{
			name:     "Empty",
			steps:    []core.Step{},
			expected: false,
		},
		{
			name: "NoHITL",
			steps: []core.Step{
				{Name: "step1", ExecutorConfig: core.ExecutorConfig{Type: "command"}},
				{Name: "step2", ExecutorConfig: core.ExecutorConfig{Type: "dag"}},
			},
			expected: false,
		},
		{
			name: "HasHITL",
			steps: []core.Step{
				{Name: "step1", ExecutorConfig: core.ExecutorConfig{Type: "command"}},
				{Name: "step2", ExecutorConfig: core.ExecutorConfig{Type: "hitl"}},
			},
			expected: true,
		},
		{
			name: "OnlyHITL",
			steps: []core.Step{
				{Name: "step1", ExecutorConfig: core.ExecutorConfig{Type: "hitl"}},
			},
			expected: true,
		},
		{
			name: "EmptyType",
			steps: []core.Step{
				{Name: "step1", ExecutorConfig: core.ExecutorConfig{Type: ""}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{Steps: tt.steps}
			result := dag.HasHITLSteps()
			assert.Equal(t, tt.expected, result)
		})
	}
}
