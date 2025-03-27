package jsondb

import (
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/stretchr/testify/require"
)

func TestExecution(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-id-1", NewUTC(time.Now()))

		ts1 := NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))
		ts3 := NewUTC(time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC))

		_ = exec.WriteStatus(t, ts1, scheduler.StatusRunning)
		_ = exec.WriteStatus(t, ts2, scheduler.StatusSuccess)
		_ = exec.WriteStatus(t, ts3, scheduler.StatusError)

		record, err := exec.LatestRecord(exec.Context, nil)
		require.NoError(t, err)

		status, err := record.ReadStatus(exec.Context)
		require.NoError(t, err)

		require.Equal(t, scheduler.StatusError.String(), status.Status.String())
	})
	t.Run("LastUpdated", func(t *testing.T) {
		root := setupTestDataRoot(t)
		exec := root.CreateTestExecution(t, "test-id-1", NewUTC(time.Now()))

		ts1 := NewUTC(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		ts2 := NewUTC(time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC))

		_ = exec.WriteStatus(t, ts1, scheduler.StatusRunning)
		record := exec.WriteStatus(t, ts2, scheduler.StatusSuccess)

		lastUpdate, err := exec.LastUpdated(exec.Context)
		require.NoError(t, err)

		info, err := os.Stat(record.file)
		require.NoError(t, err)

		require.Equal(t, info.ModTime(), lastUpdate)
	})
}

type ExecutionTest struct {
	DataRootTest
	*Execution
	TB testing.TB
}

func (et ExecutionTest) WriteStatus(t *testing.T, ts TimeInUTC, s scheduler.Status) *Record {
	t.Helper()

	dag := &digraph.DAG{Name: "test-dag"}
	status := persistence.NewStatusFactory(dag).Default()
	status.RequestID = "test-id-1"
	status.Status = s

	record, err := et.CreateRecord(et.Context, ts, nil)
	require.NoError(t, err)
	err = record.Open(et.Context)
	require.NoError(t, err)

	defer func() {
		_ = record.Close(et.Context)
	}()

	err = record.Write(et.Context, status)
	require.NoError(t, err)

	return record
}
