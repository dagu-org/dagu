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

		// Verify that the hash is based on the combined name+dagRunID
		name := "testdag"
		runID := "testrun"

		addr := core.SockAddr(name, runID)

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

	t.Run("SockAddrForChildDAGRun", func(t *testing.T) {
		t.Parallel()

		// Test SockAddrForChildDAGRun always uses GetName() and dagRunID
		dag := &core.DAG{
			Name:     "parentdag",
			Location: "path/to/parent.yml",
		}

		childRunID := "child-run-456"
		addr := dag.SockAddrForChildDAGRun(childRunID)

		// Should use the DAG name (not location) with the child run ID
		expectedAddr := core.SockAddr("parentdag", childRunID)
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

func TestAuthConfig(t *testing.T) {
	t.Run("AuthConfigFields", func(t *testing.T) {
		auth := &core.AuthConfig{
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
		dag := &core.DAG{
			Name: "test-dag",
			RegistryAuths: map[string]*core.AuthConfig{
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
