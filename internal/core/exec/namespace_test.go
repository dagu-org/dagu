package exec

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamespaceDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		baseDir string
		nsID    string
		want    string
	}{
		{"/data", "0000", filepath.Join("/data", "ns", "0000")},
		{"/dags", "a1b2", filepath.Join("/dags", "ns", "a1b2")},
		{"/var/log", "ffff", filepath.Join("/var/log", "ns", "ffff")},
	}

	for _, tt := range tests {
		got := NamespaceDir(tt.baseDir, tt.nsID)
		assert.Equal(t, tt.want, got, "NamespaceDir(%q, %q)", tt.baseDir, tt.nsID)
	}
}
