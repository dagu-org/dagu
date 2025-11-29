//go:build !windows
// +build !windows

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVolumeSpec_Unix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected volumeSpec
	}{
		{
			name:  "absolute path",
			input: "/host/path:/container/path",
			expected: volumeSpec{
				Source: "/host/path",
				Target: "/container/path",
			},
		},
		{
			name:  "absolute path with mode",
			input: "/host/path:/container/path:ro",
			expected: volumeSpec{
				Source: "/host/path",
				Target: "/container/path",
				Mode:   "ro",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseVolumeSpec(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Source, result.Source)
			assert.Equal(t, tt.expected.Target, result.Target)
			assert.Equal(t, tt.expected.Mode, result.Mode)
		})
	}
}
