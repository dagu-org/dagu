package digraph_test

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAG(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	t.Run("String", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)
		ret := dag.String()
		require.Contains(t, ret, "Name: ")
		require.Contains(t, ret, "Step0: Name: 1")
		require.Contains(t, ret, "Command: true")
	})
}

func TestSockAddr(t *testing.T) {
	t.Parallel()

	t.Run("Location", func(t *testing.T) {
		dag := &digraph.DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu_testdata_testDag_yml_[0-9a-f]+\.sock$`, dag.SockAddr(""))
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		dag := &digraph.DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 50 is the maximum length of a unix socket address
		require.LessOrEqual(t, 50, len(dag.SockAddr("")))
		require.Equal(
			t,
			"/tmp/@dagu_testdata_testDagVeryLongNameThat_b92b71.sock",
			dag.SockAddr(""),
		)
	})
	t.Run("BasicFunctionality", func(t *testing.T) {
		t.Parallel()

		// Test basic socket address generation
		addr := digraph.SockAddr("mydag", "run123")

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
		addr2 := digraph.SockAddr("mydag", "run123")
		require.Equal(t, addr, addr2)
	})

	t.Run("DifferentInputsProduceDifferentHashes", func(t *testing.T) {
		t.Parallel()

		addr1 := digraph.SockAddr("dag1", "run1")
		addr2 := digraph.SockAddr("dag1", "run2")
		addr3 := digraph.SockAddr("dag2", "run1")

		// Different dagRunIDs should produce different addresses
		require.NotEqual(t, addr1, addr2)

		// Different names should produce different addresses
		require.NotEqual(t, addr1, addr3)
	})

	t.Run("SafeNameHandling", func(t *testing.T) {
		t.Parallel()

		// Test that unsafe characters are handled properly
		addr := digraph.SockAddr("my/dag\\with:special*chars", "run|with<>chars")

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
		addr := digraph.SockAddr(veryLongName, "run123")

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

		addr32 := digraph.SockAddr(name32, "run123")
		addr33 := digraph.SockAddr(name33, "run123")

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
		addr1 := digraph.SockAddr("", "")
		addr2 := digraph.SockAddr("dag", "")
		addr3 := digraph.SockAddr("", "run")

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

		// Verify that the hash is based on the combined name+dagRunID
		name := "testdag"
		runID := "testrun"

		addr := digraph.SockAddr(name, runID)

		// Extract the hash part
		parts := strings.Split(addr, "_")
		lastPart := parts[len(parts)-1]
		hash := strings.TrimSuffix(lastPart, ".sock")

		// The hash should be the first 6 characters of MD5(name+runID)
		expectedHash := fmt.Sprintf("%x", md5.Sum([]byte(name+runID)))[:6]
		require.Equal(t, expectedHash, hash)
	})

	t.Run("DAGMethodsUseSockAddr", func(t *testing.T) {
		t.Parallel()

		// Test DAG.SockAddr method behavior

		// When Location is set, it uses Location
		dag1 := &digraph.DAG{
			Name:     "mydag",
			Location: "path/to/dag.yml",
		}
		addr1 := dag1.SockAddr("run123")
		expectedAddr1 := digraph.SockAddr("path/to/dag.yml", "")
		require.Equal(t, expectedAddr1, addr1)

		// When Location is not set, it uses Name and dagRunID
		dag2 := &digraph.DAG{
			Name: "mydag",
		}
		addr2 := dag2.SockAddr("run123")
		expectedAddr2 := digraph.SockAddr("mydag", "run123")
		require.Equal(t, expectedAddr2, addr2)
	})

	t.Run("SockAddrForChildDAGRun", func(t *testing.T) {
		t.Parallel()

		// Test SockAddrForChildDAGRun always uses GetName() and dagRunID
		dag := &digraph.DAG{
			Name:     "parentdag",
			Location: "path/to/parent.yml",
		}

		childRunID := "child-run-456"
		addr := dag.SockAddrForChildDAGRun(childRunID)

		// Should use the DAG name (not location) with the child run ID
		expectedAddr := digraph.SockAddr("parentdag", childRunID)
		require.Equal(t, expectedAddr, addr)
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
		original := digraph.Schedule{
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
		var unmarshaled digraph.Schedule
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

		var schedule digraph.Schedule
		err := json.Unmarshal([]byte(invalidJSON), &schedule)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron expression")
	})
}

func TestDAG_CreateTask(t *testing.T) {
	t.Parallel()

	t.Run("BasicTaskCreation", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name:     "test-dag",
			YamlData: []byte("name: test-dag\nsteps:\n  - name: step1\n    command: echo hello"),
		}

		runID := "run-123"
		params := "param1=value1"
		selector := map[string]string{
			"gpu":    "true",
			"region": "us-east-1",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			runID,
			digraph.WithTaskParams(params),
			digraph.WithWorkerSelector(selector),
		)

		assert.NotNil(t, task)
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, runID, task.RootDagRunId)
		assert.Equal(t, coordinatorv1.Operation_OPERATION_START, task.Operation)
		assert.Equal(t, runID, task.DagRunId)
		assert.Equal(t, "test-dag", task.Target)
		assert.Equal(t, params, task.Params)
		assert.Equal(t, selector, task.WorkerSelector)
		assert.Equal(t, string(dag.YamlData), task.Definition)
		// Parent fields should be empty when no options provided
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("WithRootDagRunOption", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "child-dag",
		}

		rootRef := digraph.DAGRunRef{
			Name: "root-dag",
			ID:   "root-run-123",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_RETRY,
			"child-run-456",
			digraph.WithRootDagRun(rootRef),
		)

		assert.Equal(t, "root-dag", task.RootDagRunName)
		assert.Equal(t, "root-run-123", task.RootDagRunId)
		assert.Equal(t, "child-run-456", task.DagRunId)
		assert.Equal(t, "child-dag", task.Target)
	})

	t.Run("WithParentDagRunOption", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "child-dag",
		}

		parentRef := digraph.DAGRunRef{
			Name: "parent-dag",
			ID:   "parent-run-789",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"child-run-456",
			digraph.WithParentDagRun(parentRef),
		)

		assert.Equal(t, "parent-dag", task.ParentDagRunName)
		assert.Equal(t, "parent-run-789", task.ParentDagRunId)
		assert.Equal(t, "child-dag", task.RootDagRunName)
		assert.Equal(t, "child-run-456", task.RootDagRunId)
	})

	t.Run("WithMultipleOptions", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name:     "grandchild-dag",
			YamlData: []byte("name: grandchild-dag"),
		}

		rootRef := digraph.DAGRunRef{
			Name: "root-dag",
			ID:   "root-run-123",
		}
		parentRef := digraph.DAGRunRef{
			Name: "parent-dag",
			ID:   "parent-run-456",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"grandchild-run-789",
			digraph.WithTaskParams("nested=true"),
			digraph.WithWorkerSelector(map[string]string{"env": "prod"}),
			digraph.WithRootDagRun(rootRef),
			digraph.WithParentDagRun(parentRef),
		)

		assert.Equal(t, "root-dag", task.RootDagRunName)
		assert.Equal(t, "root-run-123", task.RootDagRunId)
		assert.Equal(t, "parent-dag", task.ParentDagRunName)
		assert.Equal(t, "parent-run-456", task.ParentDagRunId)
		assert.Equal(t, "grandchild-run-789", task.DagRunId)
		assert.Equal(t, "grandchild-dag", task.Target)
		assert.Equal(t, "nested=true", task.Params)
		assert.Equal(t, map[string]string{"env": "prod"}, task.WorkerSelector)
	})

	t.Run("EmptyWorkerSelector", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "test-dag",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
		)

		assert.Nil(t, task.WorkerSelector)
	})

	t.Run("OptionsWithEmptyRefs", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "test-dag",
		}

		// Test that empty refs don't modify the task
		emptyRootRef := digraph.DAGRunRef{}
		emptyParentRef := digraph.DAGRunRef{Name: "", ID: ""}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
			digraph.WithRootDagRun(emptyRootRef),
			digraph.WithParentDagRun(emptyParentRef),
		)

		// Should use DAG name and runID as root values
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, "run-123", task.RootDagRunId)
		// Parent fields should remain empty
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("PartiallyEmptyRefs", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "test-dag",
		}

		// Test refs with only one field set
		partialRootRef := digraph.DAGRunRef{Name: "root-dag", ID: ""}
		partialParentRef := digraph.DAGRunRef{Name: "", ID: "parent-id"}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
			digraph.WithRootDagRun(partialRootRef),
			digraph.WithParentDagRun(partialParentRef),
		)

		// Partial refs should not modify the task
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, "run-123", task.RootDagRunId)
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("CustomTaskOption", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "test-dag",
		}

		// Create a custom task option
		withStep := func(step string) digraph.TaskOption {
			return func(task *coordinatorv1.Task) {
				task.Step = step
			}
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_RETRY,
			"run-123",
			withStep("step-2"),
		)

		assert.Equal(t, "step-2", task.Step)
		assert.Equal(t, coordinatorv1.Operation_OPERATION_RETRY, task.Operation)
	})

	t.Run("NilYamlData", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name:     "test-dag",
			YamlData: nil,
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
		)

		assert.Empty(t, task.Definition)
	})

	t.Run("AllOperationTypes", func(t *testing.T) {
		t.Parallel()

		dag := &digraph.DAG{
			Name: "test-dag",
		}

		operations := []coordinatorv1.Operation{
			coordinatorv1.Operation_OPERATION_UNSPECIFIED,
			coordinatorv1.Operation_OPERATION_START,
			coordinatorv1.Operation_OPERATION_RETRY,
		}

		for _, op := range operations {
			task := dag.CreateTask(op, "run-123")
			assert.Equal(t, op, task.Operation)
		}
	})
}

func TestTaskOption_Functions(t *testing.T) {
	t.Parallel()

	t.Run("WithRootDagRun", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}
		ref := digraph.DAGRunRef{Name: "root", ID: "123"}

		digraph.WithRootDagRun(ref)(task)

		assert.Equal(t, "root", task.RootDagRunName)
		assert.Equal(t, "123", task.RootDagRunId)
	})

	t.Run("WithParentDagRun", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}
		ref := digraph.DAGRunRef{Name: "parent", ID: "456"}

		digraph.WithParentDagRun(ref)(task)

		assert.Equal(t, "parent", task.ParentDagRunName)
		assert.Equal(t, "456", task.ParentDagRunId)
	})

	t.Run("WithTaskParams", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}

		digraph.WithTaskParams("key1=value1 key2=value2")(task)

		assert.Equal(t, "key1=value1 key2=value2", task.Params)
	})

	t.Run("WithWorkerSelector", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}
		selector := map[string]string{
			"gpu":    "true",
			"region": "us-west-2",
		}

		digraph.WithWorkerSelector(selector)(task)

		assert.Equal(t, selector, task.WorkerSelector)
	})

	t.Run("WithStep", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}

		digraph.WithStep("step-name")(task)

		assert.Equal(t, "step-name", task.Step)
	})
}

func TestNextRun(t *testing.T) {
	t.Parallel()

	dag := &digraph.DAG{
		Schedule: []digraph.Schedule{
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

func TestAuthConfig(t *testing.T) {
	t.Run("AuthConfigFields", func(t *testing.T) {
		auth := &digraph.AuthConfig{
			Username: "test-user",
			Password: "test-pass",
			Auth:     "dGVzdC11c2VyOnRlc3QtcGFzcw==", // base64("test-user:test-pass")
		}

		assert.Equal(t, "test-user", auth.Username)
		assert.Equal(t, "test-pass", auth.Password)
		assert.Equal(t, "dGVzdC11c2VyOnRlc3QtcGFzcw==", auth.Auth)
	})
}

func TestDAGRegistryAuths(t *testing.T) {
	t.Run("DAGWithRegistryAuths", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "test-dag",
			RegistryAuths: map[string]*digraph.AuthConfig{
				"docker.io": {
					Username: "docker-user",
					Password: "docker-pass",
				},
				"ghcr.io": {
					Username: "github-user",
					Password: "github-token",
				},
			},
		}

		assert.NotNil(t, dag.RegistryAuths)
		assert.Len(t, dag.RegistryAuths, 2)

		dockerAuth, exists := dag.RegistryAuths["docker.io"]
		assert.True(t, exists)
		assert.Equal(t, "docker-user", dockerAuth.Username)

		ghcrAuth, exists := dag.RegistryAuths["ghcr.io"]
		assert.True(t, exists)
		assert.Equal(t, "github-user", ghcrAuth.Username)
	})
}
