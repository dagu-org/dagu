package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVolumeSpec_Common(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    volumeSpec
		expectError bool
	}{
		{
			name:  "simple volume",
			input: "myvolume:/data",
			expected: volumeSpec{
				Source: "myvolume",
				Target: "/data",
			},
		},
		{
			name:  "volume with ro mode",
			input: "myvolume:/data:ro",
			expected: volumeSpec{
				Source: "myvolume",
				Target: "/data",
				Mode:   "ro",
			},
		},
		{
			name:  "volume with rw mode",
			input: "myvolume:/data:rw",
			expected: volumeSpec{
				Source: "myvolume",
				Target: "/data",
				Mode:   "rw",
			},
		},
		{
			name:        "missing target",
			input:       "myvolume",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseVolumeSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Source, result.Source)
			assert.Equal(t, tt.expected.Target, result.Target)
			assert.Equal(t, tt.expected.Mode, result.Mode)
		})
	}
}
