package dagindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	indexv1 "github.com/dagu-org/dagu/proto/index/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestLoad_MissingFile(t *testing.T) {
	entries := Load("/nonexistent/.dag.index", nil, nil)
	assert.Nil(t, entries)
}

func TestLoad_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)
	require.NoError(t, os.WriteFile(indexPath, []byte("corrupt data"), 0600))

	entries := Load(indexPath, nil, nil)
	assert.Nil(t, entries)
}

func TestLoad_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{Version: 999}
	data, err := proto.Marshal(idx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, data, 0600))

	entries := Load(indexPath, nil, nil)
	assert.Nil(t, entries)
}

func TestLoad_FileCountMismatch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{
		Version: IndexVersion,
		Entries: []*indexv1.DAGIndexEntry{
			{FilePath: "a.yaml", FileSize: 10, ModTime: 100},
		},
	}
	data, err := proto.Marshal(idx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, data, 0600))

	// No YAML files → count mismatch
	entries := Load(indexPath, nil, nil)
	assert.Nil(t, entries)
}

func TestLoad_SizeMismatch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{
		Version: IndexVersion,
		Entries: []*indexv1.DAGIndexEntry{
			{FilePath: "a.yaml", FileSize: 999, ModTime: 100},
		},
	}
	data, err := proto.Marshal(idx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, data, 0600))

	files := []YAMLFileMeta{{Name: "a.yaml", Size: 10, ModTime: 100}}
	entries := Load(indexPath, files, nil)
	assert.Nil(t, entries)
}

func TestLoad_ModTimeMismatch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{
		Version: IndexVersion,
		Entries: []*indexv1.DAGIndexEntry{
			{FilePath: "a.yaml", FileSize: 10, ModTime: 100},
		},
	}
	data, err := proto.Marshal(idx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, data, 0600))

	files := []YAMLFileMeta{{Name: "a.yaml", Size: 10, ModTime: 200}}
	entries := Load(indexPath, files, nil)
	assert.Nil(t, entries)
}

func TestLoad_SuspendMismatch(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{
		Version: IndexVersion,
		Entries: []*indexv1.DAGIndexEntry{
			{FilePath: "a.yaml", FileSize: 10, ModTime: 100, Name: "a", Suspended: false},
		},
	}
	data, err := proto.Marshal(idx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, data, 0600))

	files := []YAMLFileMeta{{Name: "a.yaml", Size: 10, ModTime: 100}}
	flags := SuspendFlags{"a.suspend": {}}
	entries := Load(indexPath, files, flags)
	assert.Nil(t, entries)
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{
		Version: IndexVersion,
		Entries: []*indexv1.DAGIndexEntry{
			{
				FilePath:  "a.yaml",
				FileSize:  10,
				ModTime:   100,
				Name:      "a",
				Tags:      []string{"env=prod"},
				Suspended: true,
			},
		},
	}
	data, err := proto.Marshal(idx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(indexPath, data, 0600))

	files := []YAMLFileMeta{{Name: "a.yaml", Size: 10, ModTime: 100}}
	flags := SuspendFlags{"a.suspend": {}}
	entries := Load(indexPath, files, flags)
	require.Len(t, entries, 1)
	assert.Equal(t, "a", entries[0].Name)
	assert.Equal(t, []string{"env=prod"}, entries[0].Tags)
	assert.True(t, entries[0].Suspended)
}

func TestBuild_BasicDAGs(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal DAG YAML file.
	dagContent := []byte("name: test-dag\ngroup: mygroup\ndescription: a test dag\ntags:\n  - env=prod\nsteps:\n  - name: step1\n    command: echo hello\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), dagContent, 0600))

	info, err := os.Stat(filepath.Join(dir, "test.yaml"))
	require.NoError(t, err)

	files := []YAMLFileMeta{
		{Name: "test.yaml", Size: info.Size(), ModTime: info.ModTime().UnixNano()},
	}

	// Register executor capabilities for testing.
	core.RegisterExecutorCapabilities("", core.ExecutorCapabilities{
		Command: true, MultipleCommands: true, Script: true, Shell: true,
	})

	idx := Build(context.Background(), dir, files, nil)
	require.Len(t, idx.Entries, 1)

	entry := idx.Entries[0]
	assert.Equal(t, "test-dag", entry.Name)
	assert.Equal(t, "mygroup", entry.Group)
	assert.Equal(t, "a test dag", entry.Description)
	assert.Contains(t, entry.Tags, "env=prod")
	assert.Empty(t, entry.LoadError)
}

func TestBuild_WithBuildErrors(t *testing.T) {
	dir := t.TempDir()

	// Create an invalid YAML file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("not: valid: yaml: ["), 0600))

	info, err := os.Stat(filepath.Join(dir, "bad.yaml"))
	require.NoError(t, err)

	files := []YAMLFileMeta{
		{Name: "bad.yaml", Size: info.Size(), ModTime: info.ModTime().UnixNano()},
	}

	idx := Build(context.Background(), dir, files, nil)
	require.Len(t, idx.Entries, 1)
	assert.NotEmpty(t, idx.Entries[0].LoadError)
}

func TestWrite_Atomic(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, IndexFileName)

	idx := &indexv1.DAGIndex{
		Version:     IndexVersion,
		BuiltAtUnix: time.Now().Unix(),
		Entries: []*indexv1.DAGIndexEntry{
			{FilePath: "a.yaml", Name: "a"},
		},
	}

	require.NoError(t, Write(indexPath, idx))

	// Verify file exists and can be read back.
	data, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	var loaded indexv1.DAGIndex
	require.NoError(t, proto.Unmarshal(data, &loaded))
	assert.Equal(t, uint32(IndexVersion), loaded.Version)
	require.Len(t, loaded.Entries, 1)
	assert.Equal(t, "a", loaded.Entries[0].Name)
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Create a DAG YAML file.
	dagContent := []byte("name: roundtrip\ntags:\n  - team=backend\nschedule:\n  - \"0 * * * *\"\nsteps:\n  - name: s1\n    command: echo ok\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rt.yaml"), dagContent, 0600))

	core.RegisterExecutorCapabilities("", core.ExecutorCapabilities{
		Command: true, MultipleCommands: true, Script: true, Shell: true,
	})

	info, err := os.Stat(filepath.Join(dir, "rt.yaml"))
	require.NoError(t, err)

	files := []YAMLFileMeta{
		{Name: "rt.yaml", Size: info.Size(), ModTime: info.ModTime().UnixNano()},
	}

	// Build and write.
	idx := Build(context.Background(), dir, files, nil)
	indexPath := filepath.Join(dir, IndexFileName)
	require.NoError(t, Write(indexPath, idx))

	// Load and validate.
	entries := Load(indexPath, files, nil)
	require.Len(t, entries, 1)
	assert.Equal(t, "roundtrip", entries[0].Name)
	assert.Contains(t, entries[0].Tags, "team=backend")
	assert.Equal(t, "0 * * * *", entries[0].Schedule)
}

func TestDAGFromEntry(t *testing.T) {
	entry := &indexv1.DAGIndexEntry{
		FilePath:    "my-dag.yaml",
		Name:        "my-dag",
		Group:       "group1",
		Description: "desc",
		Tags:        []string{"env=prod", "critical"},
		Schedule:    "0 * * * *",
		Suspended:   true,
		LoadError:   "",
	}

	dag := DAGFromEntry(entry, "/dags")
	assert.Equal(t, "my-dag", dag.Name)
	assert.Equal(t, "/dags/my-dag.yaml", dag.Location)
	assert.Equal(t, "group1", dag.Group)
	assert.Equal(t, "desc", dag.Description)
	assert.Len(t, dag.Tags, 2)
	assert.Len(t, dag.Schedule, 1)
	assert.Equal(t, "0 * * * *", dag.Schedule[0].Expression)
	assert.NotNil(t, dag.Schedule[0].Parsed)
	assert.Empty(t, dag.BuildErrors)
}

func TestDAGFromEntry_WithError(t *testing.T) {
	entry := &indexv1.DAGIndexEntry{
		FilePath:  "bad.yaml",
		Name:      "bad",
		LoadError: "parse failed",
	}

	dag := DAGFromEntry(entry, "/dags")
	assert.Equal(t, "bad", dag.Name)
	require.Len(t, dag.BuildErrors, 1)
	assert.Equal(t, "parse failed", dag.BuildErrors[0].Error())
}

func TestBuild_WithSuspendFlags(t *testing.T) {
	dir := t.TempDir()

	dagContent := []byte("name: flagged-dag\nsteps:\n  - name: step1\n    command: echo hello\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "flagged.yaml"), dagContent, 0600))

	core.RegisterExecutorCapabilities("", core.ExecutorCapabilities{
		Command: true, MultipleCommands: true, Script: true, Shell: true,
	})

	info, err := os.Stat(filepath.Join(dir, "flagged.yaml"))
	require.NoError(t, err)

	files := []YAMLFileMeta{
		{Name: "flagged.yaml", Size: info.Size(), ModTime: info.ModTime().UnixNano()},
	}

	flags := SuspendFlags{"flagged-dag.suspend": {}}
	idx := Build(context.Background(), dir, files, flags)
	require.Len(t, idx.Entries, 1)
	assert.True(t, idx.Entries[0].Suspended)
}

func TestBuild_ContextCancellation(t *testing.T) {
	dir := t.TempDir()

	core.RegisterExecutorCapabilities("", core.ExecutorCapabilities{
		Command: true, MultipleCommands: true, Script: true, Shell: true,
	})

	// Create multiple DAG YAML files.
	var files []YAMLFileMeta
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("dag-%d.yaml", i)
		content := []byte(fmt.Sprintf("name: dag-%d\nsteps:\n  - name: s1\n    command: echo ok\n", i))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0600))
		info, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err)
		files = append(files, YAMLFileMeta{Name: name, Size: info.Size(), ModTime: info.ModTime().UnixNano()})
	}

	// Cancel context immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	idx := Build(ctx, dir, files, nil)
	// With cancelled context, should have fewer entries than files.
	assert.Less(t, len(idx.Entries), len(files))
}

func TestDAGFromEntry_MultipleSchedules(t *testing.T) {
	entry := &indexv1.DAGIndexEntry{
		FilePath: "multi.yaml",
		Name:     "multi",
		Schedule: "0 * * * *; 30 * * * *",
	}

	dag := DAGFromEntry(entry, "/dags")
	require.Len(t, dag.Schedule, 2)
	assert.Equal(t, "0 * * * *", dag.Schedule[0].Expression)
	assert.NotNil(t, dag.Schedule[0].Parsed)
	assert.Equal(t, "30 * * * *", dag.Schedule[1].Expression)
	assert.NotNil(t, dag.Schedule[1].Parsed)
}

func TestDAGFromEntry_InvalidCron(t *testing.T) {
	entry := &indexv1.DAGIndexEntry{
		FilePath: "invalid-cron.yaml",
		Name:     "invalid-cron",
		Schedule: "not-a-cron",
	}

	dag := DAGFromEntry(entry, "/dags")
	require.Len(t, dag.Schedule, 1)
	assert.Equal(t, "not-a-cron", dag.Schedule[0].Expression)
	assert.Nil(t, dag.Schedule[0].Parsed) // Non-fatal: no parsed cron
}

func TestDAGFromEntry_NoSchedule(t *testing.T) {
	entry := &indexv1.DAGIndexEntry{
		FilePath: "no-sched.yaml",
		Name:     "no-sched",
		Schedule: "",
	}

	dag := DAGFromEntry(entry, "/dags")
	assert.Nil(t, dag.Schedule)
}

func TestSuspendFlagName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"NormalName", "my-dag", "my-dag.suspend"},
		{"NameWithSpaces", "my dag", "my-dag.suspend"},
		{"NameWithSpecialChars", "my<dag>", "my-dag-.suspend"},
		{"EmptyName", "", ".suspend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SuspendFlagName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
