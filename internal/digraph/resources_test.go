package digraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCPUToMillis(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		{"empty", "", 0, false},
		{"whole number", "2", 2000, false},
		{"decimal", "0.5", 500, false},
		{"decimal with precision", "1.75", 1750, false},
		{"millicores", "500m", 500, false},
		{"millicores whole", "2000m", 2000, false},
		{"millicores decimal", "250m", 250, false},
		{"with spaces", " 1.5 ", 1500, false},
		{"invalid", "abc", 0, true},
		{"invalid suffix", "2x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCPUToMillis(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		// Empty and basic
		{"empty", "", 0, false},
		{"bytes no unit", "1024", 1024, false},
		
		// Binary units
		{"kibibytes lower", "1ki", Kibibyte, false},
		{"kibibytes upper", "1Ki", Kibibyte, false},
		{"kibibytes full", "1KiB", Kibibyte, false},
		{"mebibytes", "2Mi", 2 * Mebibyte, false},
		{"gibibytes", "4Gi", 4 * Gibibyte, false},
		{"tebibytes", "1Ti", Tebibyte, false},
		
		// Decimal units
		{"kilobytes lower", "1k", Kilobyte, false},
		{"kilobytes upper", "1K", Kilobyte, false},
		{"kilobytes full", "1KB", Kilobyte, false},
		{"megabytes", "2M", 2 * Megabyte, false},
		{"gigabytes", "4G", 4 * Gigabyte, false},
		{"terabytes", "1T", Terabyte, false},
		
		// Decimal values
		{"decimal kibibytes", "1.5Ki", int64(1.5 * Kibibyte), false},
		{"decimal gigabytes", "2.5Gi", int64(2.5 * Gibibyte), false},
		
		// With spaces
		{"with spaces", " 100Mi ", 100 * Mebibyte, false},
		
		// Invalid
		{"invalid", "abc", 0, true},
		{"invalid unit", "100X", 0, true},
		{"no number", "Gi", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemory(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestParseResourcesConfig(t *testing.T) {
	tests := []struct {
		name               string
		input              *ResourcesConfig
		wantCPURequest     int
		wantCPULimit       int
		wantMemRequest     int64
		wantMemLimit       int64
		wantErr            bool
	}{
		{
			name:           "nil config",
			input:          nil,
			wantCPURequest: 0,
			wantCPULimit:   0,
			wantMemRequest: 0,
			wantMemLimit:   0,
			wantErr:        false,
		},
		{
			name: "requests and limits",
			input: &ResourcesConfig{
				Requests: &ResourceQuantities{
					CPU:    "0.5",
					Memory: "1Gi",
				},
				Limits: &ResourceQuantities{
					CPU:    "2",
					Memory: "4Gi",
				},
			},
			wantCPURequest: 500,
			wantCPULimit:   2000,
			wantMemRequest: Gibibyte,
			wantMemLimit:   4 * Gibibyte,
			wantErr:        false,
		},
		{
			name: "millicores",
			input: &ResourcesConfig{
				Requests: &ResourceQuantities{
					CPU: "250m",
				},
				Limits: &ResourceQuantities{
					CPU: "1500m",
				},
			},
			wantCPURequest: 250,
			wantCPULimit:   1500,
			wantErr:        false,
		},
		{
			name: "only limits",
			input: &ResourcesConfig{
				Limits: &ResourceQuantities{
					CPU:    "4",
					Memory: "8Gi",
				},
			},
			wantCPULimit: 4000,
			wantMemLimit: 8 * Gibibyte,
			wantErr:      false,
		},
		{
			name: "invalid CPU",
			input: &ResourcesConfig{
				Requests: &ResourceQuantities{
					CPU: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid memory",
			input: &ResourcesConfig{
				Limits: &ResourceQuantities{
					Memory: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResourcesConfig(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantCPURequest, got.CPURequestMillis)
				assert.Equal(t, tt.wantCPULimit, got.CPULimitMillis)
				assert.Equal(t, tt.wantMemRequest, got.MemoryRequestBytes)
				assert.Equal(t, tt.wantMemLimit, got.MemoryLimitBytes)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero", 0, "0"},
		{"bytes", 100, "100"},
		{"kibibytes exact", Kibibyte, "1Ki"},
		{"kibibytes decimal", int64(1.5 * Kibibyte), "1.50Ki"},
		{"mebibytes", 2 * Mebibyte, "2Mi"},
		{"gibibytes", 4 * Gibibyte, "4Gi"},
		{"tebibytes", Tebibyte, "1Ti"},
		{"mixed size", 1536 * Mebibyte, "1.50Gi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{"zero", 0, "0"},
		{"whole number", 2.0, "2"},
		{"decimal", 0.5, "0.50"},
		{"decimal precision", 1.75, "1.75"},
		{"large whole", 16.0, "16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCPU(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFormatCPUMillis(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"zero", 0, "0"},
		{"whole cores", 2000, "2"},
		{"half core", 500, "0.5"},
		{"quarter core", 250, "0.25"},
		{"1.75 cores", 1750, "1.75"},
		{"100 millicores", 100, "0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCPUMillis(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}