package persis

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory("/config", "/data", "/logs")
	require.NotNil(t, f)
	assert.Equal(t, "/config", f.ConfigDir())
	assert.Equal(t, "/data", f.DataDir())
	assert.Equal(t, "/logs", f.LogDir())
}

func TestFactory_PathAccessors(t *testing.T) {
	f := NewFactory("/config", "/data", "/logs")
	nsID := "550e8400-e29b-41d4-a716-446655440000"

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "NamespacesDir",
			fn:       f.NamespacesDir,
			expected: filepath.Join("/config", "namespaces"),
		},
		{
			name:     "NamespaceConfigDir",
			fn:       func() string { return f.NamespaceConfigDir(nsID) },
			expected: filepath.Join("/config", "namespaces", nsID),
		},
		{
			name:     "NamespaceConfigFile",
			fn:       func() string { return f.NamespaceConfigFile(nsID) },
			expected: filepath.Join("/config", "namespaces", nsID, "config.yaml"),
		},
		{
			name:     "NamespaceBaseConfigFile",
			fn:       func() string { return f.NamespaceBaseConfigFile(nsID) },
			expected: filepath.Join("/config", "namespaces", nsID, "base.yaml"),
		},
		{
			name:     "NamespaceDataDir",
			fn:       func() string { return f.NamespaceDataDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID),
		},
		{
			name:     "DAGsDir",
			fn:       func() string { return f.DAGsDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "dags"),
		},
		{
			name:     "DAGRunsDir",
			fn:       func() string { return f.DAGRunsDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "dag-runs"),
		},
		{
			name:     "QueueDir",
			fn:       func() string { return f.QueueDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "queue"),
		},
		{
			name:     "ProcsDir",
			fn:       func() string { return f.ProcsDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "procs"),
		},
		{
			name:     "FlagsDir",
			fn:       func() string { return f.FlagsDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "flags"),
		},
		{
			name:     "LogsDir",
			fn:       func() string { return f.LogsDir(nsID) },
			expected: filepath.Join("/logs", "namespaces", nsID),
		},
		{
			name:     "AuditDir",
			fn:       func() string { return f.AuditDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "audit"),
		},
		{
			name:     "WebhooksDir",
			fn:       func() string { return f.WebhooksDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "webhooks"),
		},
		{
			name:     "APIKeysDir",
			fn:       func() string { return f.APIKeysDir(nsID) },
			expected: filepath.Join("/data", "namespaces", nsID, "apikeys"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.fn())
		})
	}
}

func TestFactory_ForNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFactory(
		filepath.Join(tmpDir, "config"),
		filepath.Join(tmpDir, "data"),
		filepath.Join(tmpDir, "logs"),
	)

	nsID := "test-namespace-id"
	stores := f.ForNamespace(nsID)

	require.NotNil(t, stores)
	require.NotNil(t, stores.DAGs)
	require.NotNil(t, stores.DAGRuns)
	require.NotNil(t, stores.Queue)
	require.NotNil(t, stores.Procs)
}

func TestFactory_String(t *testing.T) {
	f := NewFactory("/config", "/data", "/logs")
	s := f.String()
	assert.Contains(t, s, "/config")
	assert.Contains(t, s, "/data")
	assert.Contains(t, s, "/logs")
}
