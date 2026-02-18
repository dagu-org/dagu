package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNavigateDeepPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		want    []string
	}{
		{
			name: "root level",
			path: "",
			want: []string{"Properties:", "name", "steps", "schedule"},
		},
		{
			name: "steps (oneOf)",
			path: "steps",
			want: []string{"oneOf", "array", "object"},
		},
		{
			name: "steps.name (through oneOf and ref)",
			path: "steps.name",
			want: []string{"string", "Unique identifier"},
		},
		{
			name: "steps.container (nested oneOf)",
			path: "steps.container",
			want: []string{"oneOf", "image", "volumes"},
		},
		{
			name: "schedule (oneOf with object)",
			path: "schedule",
			want: []string{"oneOf", "cron", "start", "stop"},
		},
		{
			name: "handler_on.success",
			path: "handler_on.success",
			want: []string{"object"},
		},
		{
			name:    "invalid path",
			path:    "nonexistent.field",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := DefaultRegistry.Navigate("dag", tt.path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.want {
				assert.Contains(t, result, want)
			}
		})
	}
}
