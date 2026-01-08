package docker

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

func TestDockerConfigSchema(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name:    "image only",
			config:  map[string]any{"image": "alpine"},
			wantErr: false,
		},
		{
			name:    "containerName only",
			config:  map[string]any{"containerName": "my-container"},
			wantErr: false,
		},
		{
			name:    "both image and containerName",
			config:  map[string]any{"image": "alpine", "containerName": "my-container"},
			wantErr: false,
		},
		{
			name:    "image with exec requires containerName",
			config:  map[string]any{"image": "alpine", "exec": map[string]any{"user": "root"}},
			wantErr: true,
		},
		{
			name:    "containerName with exec",
			config:  map[string]any{"containerName": "my-container", "exec": map[string]any{"user": "root"}},
			wantErr: false,
		},
		{
			name:    "empty config",
			config:  map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ValidateExecutorConfig("docker", tt.config)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
