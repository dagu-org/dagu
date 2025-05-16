package localhistory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type LocalStorageTest struct {
	Context     context.Context
	HistoryRepo models.HistoryRepository
	TmpDir      string
}

func setupTestLocalStorage(t *testing.T) LocalStorageTest {
	tmpDir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)

	th := LocalStorageTest{
		Context:     context.Background(),
		HistoryRepo: New(tmpDir),
		TmpDir:      tmpDir,
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(th.TmpDir)
	})
	return th
}

func (th LocalStorageTest) CreateRun(t *testing.T, ts time.Time, workflowID string, s scheduler.Status) *Run {
	t.Helper()

	dag := th.DAG("test_DAG")
	run, err := th.HistoryRepo.CreateRun(th.Context, dag.DAG, ts, workflowID, models.NewRunOptions{})
	require.NoError(t, err)

	err = run.Open(th.Context)
	require.NoError(t, err)

	defer func() {
		_ = run.Close(th.Context)
	}()

	status := models.InitialStatus(dag.DAG)
	status.WorkflowID = workflowID
	status.Status = s

	err = run.Write(th.Context, status)
	require.NoError(t, err)

	return run.(*Run)
}

func (th LocalStorageTest) DAG(name string) DAGTest {
	return DAGTest{
		th: th,
		DAG: &digraph.DAG{
			Name:     name,
			Location: filepath.Join(th.TmpDir, name+".yaml"),
		},
	}
}

type DAGTest struct {
	th LocalStorageTest
	*digraph.DAG
}

func (d DAGTest) Writer(t *testing.T, workflowID string, startedAt time.Time) WriterTest {
	t.Helper()

	root := NewDataRoot(d.th.TmpDir, d.Name)
	workflow, err := root.CreateWorkflow(models.NewUTC(startedAt), workflowID)
	require.NoError(t, err)

	obj := d.th.HistoryRepo.(*localStorage)
	run, err := workflow.CreateRun(d.th.Context, models.NewUTC(startedAt), obj.cache, WithDAG(d.DAG))
	require.NoError(t, err)

	writer := NewWriter(run.file)
	require.NoError(t, writer.Open())

	t.Cleanup(func() {
		require.NoError(t, writer.close())
	})

	return WriterTest{
		th: d.th,

		WorkflowID: workflowID,
		FilePath:   run.file,
		Writer:     writer,
	}
}

func (w WriterTest) Write(t *testing.T, status models.Status) {
	t.Helper()

	err := w.Writer.write(status)
	require.NoError(t, err)
}

func (w WriterTest) AssertContent(t *testing.T, name, workflowID string, status scheduler.Status) {
	t.Helper()

	data, err := ParseStatusFile(w.FilePath)
	require.NoError(t, err)

	assert.Equal(t, name, data.Name)
	assert.Equal(t, workflowID, data.WorkflowID)
	assert.Equal(t, status, data.Status)
}

func (w WriterTest) Close(t *testing.T) {
	t.Helper()

	require.NoError(t, w.Writer.close())
}

type WriterTest struct {
	th LocalStorageTest

	WorkflowID string
	FilePath   string
	Writer     *Writer
	Closed     bool
}
