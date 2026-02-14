package schema

import (
	"strings"
	"testing"
)

// TestNavigateDeepPaths tests navigation through complex nested schema constructs.
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Navigate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("Navigate() result missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}
