package schema

import (
	"strings"
	"testing"
)

// TestNavigateDeepPaths tests navigation through complex nested schema constructs.
func TestNavigateDeepPaths(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		want    []string // strings that should appear in result
	}{
		{
			name:    "root level",
			path:    "",
			wantErr: false,
			want:    []string{"Properties:", "name", "steps", "schedule"},
		},
		{
			name:    "steps (oneOf)",
			path:    "steps",
			wantErr: false,
			want:    []string{"oneOf", "array", "object"},
		},
		{
			name:    "steps.name (through oneOf and ref)",
			path:    "steps.name",
			wantErr: false,
			want:    []string{"string", "Unique identifier"},
		},
		{
			name:    "steps.container (nested oneOf)",
			path:    "steps.container",
			wantErr: false,
			want:    []string{"oneOf", "image", "volumes"},
		},
		{
			name:    "schedule (oneOf with object)",
			path:    "schedule",
			wantErr: false,
			want:    []string{"oneOf", "cron", "start", "stop"},
		},
		{
			name:    "handlerOn.success",
			path:    "handlerOn.success",
			wantErr: false,
			want:    []string{"object"},
		},
		{
			name:    "invalid path",
			path:    "nonexistent.field",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
