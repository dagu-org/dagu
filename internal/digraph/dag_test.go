package digraph_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestDAG(t *testing.T) {
	th := test.Setup(t)
	t.Run("String", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("digraph", "default.yaml"))
		ret := dag.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestUnixSocket(t *testing.T) {
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
			"/tmp/@dagu_testdata_testDagVeryLongNameThat_21bace.sock",
			dag.SockAddr(""),
		)
	})
}

func TestMarshalJSON(t *testing.T) {
	th := test.Setup(t)
	t.Run("MarshalJSON", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("digraph", "default.yaml"))
		_, err := json.Marshal(dag.DAG)
		require.NoError(t, err)
	})
}

func TestScheduleJSON(t *testing.T) {
	t.Run("MarshalUnmarshalJSON", func(t *testing.T) {
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
		// Test unmarshaling with an invalid cron expression
		invalidJSON := `{"expression":"invalid cron"}`

		var schedule digraph.Schedule
		err := json.Unmarshal([]byte(invalidJSON), &schedule)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron expression")
	})
}
