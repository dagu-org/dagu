package core_test

import (
	"crypto/md5"
	"encoding/json"
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
		// Note: Hash includes namespace prefix (empty in this case)
		require.Equal(
			t,
			"/tmp/@dagu_testdata_testDagVeryLongNameThat_e182b1.sock",
			dag.SockAddr(""),
		)
	})
	t.Run("BasicFunctionality", func(t *testing.T) {
		t.Parallel()

		// Test basic socket address generation
		addr := core.SockAddr("mydag", "run123")

		// Should start with /tmp/@dagu_
		require.True(t, strings.HasPrefix(addr, "/tmp/@dagu_"))

		// Should end with .sock
		require.True(t, strings.HasSuffix(addr, ".sock"))

		// Should contain the safe name
		require.Contains(t, addr, "mydag")

		// Should have a 6-character hash
		parts := strings.Split(addr, "_")
		lastPart := parts[len(parts)-1]
		hashPart := strings.TrimSuffix(lastPart, ".sock")
		require.Len(t, hashPart, 6)

		// Should be deterministic - same inputs produce same output
		addr2 := core.SockAddr("mydag", "run123")
		require.Equal(t, addr, addr2)
	})

	t.Run("DifferentInputsProduceDifferentHashes", func(t *testing.T) {
		t.Parallel()

		addr1 := core.SockAddr("dag1", "run1")
		addr2 := core.SockAddr("dag1", "run2")
		addr3 := core.SockAddr("dag2", "run1")

		// Different dagRunIDs should produce different addresses
		require.NotEqual(t, addr1, addr2)

		// Different names should produce different addresses
		require.NotEqual(t, addr1, addr3)
	})

	t.Run("SafeNameHandling", func(t *testing.T) {
		t.Parallel()

		// Test that unsafe characters are handled properly
		addr := core.SockAddr("my/dag\\with:special*chars", "run|with<>chars")

		// Extract just the socket name part (after /tmp/)
		socketName := strings.TrimPrefix(addr, "/tmp/")

		// Should not contain any of the unsafe characters in the socket name
		// (but "/" is expected in the full path as "/tmp/")
		require.NotContains(t, socketName, "/")
		require.NotContains(t, socketName, "\\")
		require.NotContains(t, socketName, ":")
		require.NotContains(t, socketName, "*")
		require.NotContains(t, socketName, "|")
		require.NotContains(t, socketName, "<")
		require.NotContains(t, socketName, ">")
	})

	t.Run("MaxSocketLengthEnforcement", func(t *testing.T) {
		t.Parallel()

		// Test that very long names are truncated to keep total length <= 50
		veryLongName := strings.Repeat("a", 100)
		addr := core.SockAddr(veryLongName, "run123")

		// Extract just the socket name part (after /tmp/)
		socketName := strings.TrimPrefix(addr, "/tmp/")
		require.LessOrEqual(t, len(socketName), 50)

		// Should still have all required parts
		require.True(t, strings.HasPrefix(socketName, "@dagu_"))
		require.True(t, strings.HasSuffix(socketName, ".sock"))
	})

	t.Run("EdgeCaseTruncation", func(t *testing.T) {
		t.Parallel()

		// Test edge case where name needs to be truncated to exactly fit
		// Format: @dagu_ (6) + name (?) + _ (1) + hash (6) + .sock (5) = 50
		// So max name length = 50 - 6 - 1 - 6 - 5 = 32

		// Test with name that will need truncation
		name32 := strings.Repeat("x", 32)
		name33 := strings.Repeat("x", 33)

		addr32 := core.SockAddr(name32, "run123")
		addr33 := core.SockAddr(name33, "run123")

		socketName32 := strings.TrimPrefix(addr32, "/tmp/")
		socketName33 := strings.TrimPrefix(addr33, "/tmp/")

		// Both should be exactly 50 characters or less
		require.LessOrEqual(t, len(socketName32), 50)
		require.LessOrEqual(t, len(socketName33), 50)

		// The 33-char name should be truncated
		require.Contains(t, socketName32, name32)
		require.NotContains(t, socketName33, name33) // Full name won't fit
	})

	t.Run("EmptyInputs", func(t *testing.T) {
		t.Parallel()

		// Test with empty strings
		addr1 := core.SockAddr("", "")
		addr2 := core.SockAddr("dag", "")
		addr3 := core.SockAddr("", "run")

		// All should produce valid socket addresses
		require.True(t, strings.HasPrefix(addr1, "/tmp/@dagu_"))
		require.True(t, strings.HasPrefix(addr2, "/tmp/@dagu_"))
		require.True(t, strings.HasPrefix(addr3, "/tmp/@dagu_"))

		// All should end with .sock
		require.True(t, strings.HasSuffix(addr1, ".sock"))
		require.True(t, strings.HasSuffix(addr2, ".sock"))
		require.True(t, strings.HasSuffix(addr3, ".sock"))
	})

	t.Run("HashConsistency", func(t *testing.T) {
		t.Parallel()

		// Verify that the hash is based on the combined namespace+"."+name+dagRunID
		name := "testdag"
		runID := "testrun"

		addr := core.SockAddr(name, runID)

		// Extract the hash part
		parts := strings.Split(addr, "_")
		lastPart := parts[len(parts)-1]
		hash := strings.TrimSuffix(lastPart, ".sock")

		// The hash should be the first 6 characters of MD5("." + name + runID)
		// (empty namespace results in "." prefix)
		expectedHash := fmt.Sprintf("%x", md5.Sum([]byte("."+name+runID)))[:6]
		require.Equal(t, expectedHash, hash)
	})

	t.Run("DAGMethodsUseSockAddr", func(t *testing.T) {
		t.Parallel()

		// Test DAG.SockAddr method behavior

		// When Location is set, it uses Location
		dag1 := &core.DAG{
			Name:     "mydag",
			Location: "path/to/dag.yml",
		}
		addr1 := dag1.SockAddr("run123")
		expectedAddr1 := core.SockAddr("path/to/dag.yml", "")
		require.Equal(t, expectedAddr1, addr1)

		// When Location is not set, it uses Name and dagRunID
		dag2 := &core.DAG{
			Name: "mydag",
		}
		addr2 := dag2.SockAddr("run123")
		expectedAddr2 := core.SockAddr("mydag", "run123")
		require.Equal(t, expectedAddr2, addr2)
	})

	t.Run("SockAddrForSubDAGRun", func(t *testing.T) {
		t.Parallel()

		// Test SockAddrForSubDAGRun always uses GetName() and dagRunID
		dag := &core.DAG{
			Name:     "parentdag",
			Location: "path/to/parent.yml",
		}

		subRunID := "child-run-456"
		addr := dag.SockAddrForSubDAGRun(subRunID)

		// Should use the DAG name (not location) with the sub run ID
		expectedAddr := core.SockAddr("parentdag", subRunID)
		require.Equal(t, expectedAddr, addr)
	})

	t.Run("NamespaceIsolation", func(t *testing.T) {
		t.Parallel()

		// DAGs with same name in different namespaces should have different socket addresses
		dag1 := &core.DAG{
			Name:      "mydag",
			Namespace: "team-a",
		}
		dag2 := &core.DAG{
			Name:      "mydag",
			Namespace: "team-b",
		}

		addr1 := dag1.SockAddr("run123")
		addr2 := dag2.SockAddr("run123")

		// Different namespaces should produce different addresses
		require.NotEqual(t, addr1, addr2)

		// Same namespace and name should produce same address
		dag3 := &core.DAG{
			Name:      "mydag",
			Namespace: "team-a",
		}
		addr3 := dag3.SockAddr("run123")
		require.Equal(t, addr1, addr3)
	})

	t.Run("SockAddrWithNamespace", func(t *testing.T) {
		t.Parallel()

		// Test the SockAddrWithNamespace function directly
		addr1 := core.SockAddrWithNamespace("ns1", "dag", "run")
		addr2 := core.SockAddrWithNamespace("ns2", "dag", "run")
		addr3 := core.SockAddrWithNamespace("", "dag", "run")

		// Different namespaces should produce different addresses
		require.NotEqual(t, addr1, addr2)
		require.NotEqual(t, addr1, addr3)
		require.NotEqual(t, addr2, addr3)

		// Should include namespace in the socket name
		require.Contains(t, addr1, "ns1")
		require.Contains(t, addr2, "ns2")
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

		// Create a Schedule with a valid cron expression
		original := core.Schedule{
			Expression: "0 0 * * *", // Run at midnight every day
		}

		// Parse the expression to populate the Parsed field
		parsed, err := cron.ParseStandard(original.Expression)
		require.NoError(t, err)
		original.Parsed = parsed

		// Marshal to JSON
		data, err := json.Marshal(original)
		require.NoError(t, err)

		// Verify JSON format (camelCase field names)
		jsonStr := string(data)
		require.Contains(t, jsonStr, `"expression":"0 0 * * *"`)
		require.NotContains(t, jsonStr, `"Expression"`)
		require.NotContains(t, jsonStr, `"Parsed"`)

		// Unmarshal back to a Schedule struct
		var unmarshaled core.Schedule
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		// Verify the unmarshaled struct has the correct values
		require.Equal(t, original.Expression, unmarshaled.Expression)

		// Verify the Parsed field was populated correctly
		require.NotNil(t, unmarshaled.Parsed)

		// Test that the next scheduled time is the same for both objects
		// This verifies that the Parsed field was correctly populated during unmarshaling
		now := time.Now()
		expectedNext := original.Parsed.Next(now)
		actualNext := unmarshaled.Parsed.Next(now)
		require.Equal(t, expectedNext, actualNext)
	})

	t.Run("UnmarshalInvalidCron", func(t *testing.T) {
		t.Parallel()

		// Test unmarshaling with an invalid cron expression
		invalidJSON := `{"expression":"invalid cron"}`

		var schedule core.Schedule
		err := json.Unmarshal([]byte(invalidJSON), &schedule)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron expression")
	})
}

func TestNextRun(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Schedule: []core.Schedule{
			{Expression: "0 1 * * *"}, // Daily at 1 AM
		},
	}
	parsedCron, err := cron.ParseStandard(dag.Schedule[0].Expression)
	require.NoError(t, err)
	dag.Schedule[0].Parsed = parsedCron

	now := time.Date(2023, 10, 1, 1, 0, 0, 0, time.UTC)
	nextRun := dag.NextRun(now)

	// Next run should be the next day at 1 AM
	expectedNext := time.Date(2023, 10, 2, 1, 0, 0, 0, time.UTC)
	require.Equal(t, expectedNext, nextRun)
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
		name    string
		dag     *core.DAG
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid DAG with name passes",
			dag: &core.DAG{
				Name: "test-dag",
				Steps: []core.Step{
					{Name: "step1"},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty name fails",
			dag:     &core.DAG{Name: ""},
			wantErr: true,
			errMsg:  "DAG name is required",
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
			wantErr: false,
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
			wantErr: true,
			errMsg:  "non-existent step",
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
			wantErr: false,
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.dag.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
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
			Steps: []core.Step{
				{Name: "step1"},
				{Name: "step2"},
			},
		}
		result := dag.String()

		// Verify key fields are included
		assert.Contains(t, result, "test-dag")
		assert.Contains(t, result, "A test DAG")
		assert.Contains(t, result, "param1=value1")
		assert.Contains(t, result, "/var/log/dags")
	})

	t.Run("minimal DAG basic output", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{
			Name: "minimal",
		}
		result := dag.String()
		assert.Contains(t, result, "minimal")
		assert.Contains(t, result, "{")
		assert.Contains(t, result, "}")
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
		now := time.Now()
		result := dag.NextRun(now)
		assert.True(t, result.IsZero())
	})

	t.Run("single schedule returns correct next time", func(t *testing.T) {
		t.Parallel()

		// Schedule for every hour at minute 0
		parsed, err := cron.ParseStandard("0 * * * *")
		require.NoError(t, err)

		dag := &core.DAG{
			Schedule: []core.Schedule{
				{Expression: "0 * * * *", Parsed: parsed},
			},
		}

		now := time.Date(2023, 10, 1, 12, 30, 0, 0, time.UTC)
		result := dag.NextRun(now)

		// Should be the next hour
		expected := time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC)
		assert.Equal(t, expected, result)
	})

	t.Run("multiple schedules returns earliest", func(t *testing.T) {
		t.Parallel()

		// First schedule: every hour at minute 0
		hourly, err := cron.ParseStandard("0 * * * *")
		require.NoError(t, err)

		// Second schedule: every 30 minutes
		halfHourly, err := cron.ParseStandard("*/30 * * * *")
		require.NoError(t, err)

		dag := &core.DAG{
			Schedule: []core.Schedule{
				{Expression: "0 * * * *", Parsed: hourly},
				{Expression: "*/30 * * * *", Parsed: halfHourly},
			},
		}

		now := time.Date(2023, 10, 1, 12, 15, 0, 0, time.UTC)
		result := dag.NextRun(now)

		// Should be at 12:30 (every 30 min) before 13:00 (hourly)
		expected := time.Date(2023, 10, 1, 12, 30, 0, 0, time.UTC)
		assert.Equal(t, expected, result)
	})

	t.Run("nil Parsed in schedule is skipped", func(t *testing.T) {
		t.Parallel()

		parsed, err := cron.ParseStandard("0 * * * *")
		require.NoError(t, err)

		dag := &core.DAG{
			Schedule: []core.Schedule{
				{Expression: "invalid", Parsed: nil},      // nil Parsed should be skipped
				{Expression: "0 * * * *", Parsed: parsed}, // valid
			},
		}

		now := time.Date(2023, 10, 1, 12, 30, 0, 0, time.UTC)
		result := dag.NextRun(now)

		expected := time.Date(2023, 10, 1, 13, 0, 0, 0, time.UTC)
		assert.Equal(t, expected, result)
	})
}

func TestDAG_GetName(t *testing.T) {
	t.Parallel()

	t.Run("name set returns name", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{Name: "my-dag", Location: "/path/to/other.yaml"}
		assert.Equal(t, "my-dag", dag.GetName())
	})

	t.Run("name empty returns filename from location", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{Name: "", Location: "/path/to/mydag.yaml"}
		assert.Equal(t, "mydag", dag.GetName())
	})

	t.Run("name empty and location empty returns empty", func(t *testing.T) {
		t.Parallel()
		dag := &core.DAG{Name: "", Location: ""}
		assert.Equal(t, "", dag.GetName())
	})
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
