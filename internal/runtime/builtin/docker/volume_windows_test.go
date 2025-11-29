//go:build windows
// +build windows

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVolumeSpec_Windows(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected volumeSpec
	}{
		{
			name:  "windows path with backslash",
			input: `C:\temp\data:/data`,
			expected: volumeSpec{
				Source: `C:\temp\data`,
				Target: "/data",
			},
		},
		{
			name:  "windows path with forward slash",
			input: "C:/temp/data:/data",
			expected: volumeSpec{
				Source: "C:/temp/data",
				Target: "/data",
			},
		},
		{
			name:  "windows path with mode",
			input: `C:\temp\data:/data:ro`,
			expected: volumeSpec{
				Source: `C:\temp\data`,
				Target: "/data",
				Mode:   "ro",
			},
		},
		{
			name:  "docker toolbox style path",
			input: "//C:/temp/data:/data",
			expected: volumeSpec{
				Source: "//C:/temp/data",
				Target: "/data",
			},
		},
		{
			name:  "docker toolbox style path with mode",
			input: "//C:/temp/data:/data:rw",
			expected: volumeSpec{
				Source: "//C:/temp/data",
				Target: "/data",
				Mode:   "rw",
			},
		},
		{
			name:  "D drive",
			input: "D:/projects:/app",
			expected: volumeSpec{
				Source: "D:/projects",
				Target: "/app",
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
