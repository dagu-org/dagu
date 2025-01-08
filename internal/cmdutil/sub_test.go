package cmdutil

import (
	"os"
	"runtime"
	"testing"
)

func TestSubstituteCommands(t *testing.T) {
	// Skip tests on Windows as they require different commands
	if runtime.GOOS == "windows" {
		t.Skip("Skipping tests on Windows")
	}

	tests := []struct {
		name       string
		input      string
		want       string
		wantErr    bool
		setupEnv   map[string]string
		cleanupEnv []string
		skipOnOS   []string
	}{
		{
			name:    "no command substitution needed",
			input:   "hello world",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "simple echo command",
			input:   "prefix `echo hello` suffix",
			want:    "prefix hello suffix",
			wantErr: false,
		},
		{
			name:    "multiple commands",
			input:   "`echo foo` and `echo bar`",
			want:    "foo and bar",
			wantErr: false,
		},
		{
			name:    "nested quotes",
			input:   "`echo \"hello world\"`",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "command with environment variable",
			input:   "`echo $TEST_VAR`",
			want:    "test_value",
			wantErr: false,
			setupEnv: map[string]string{
				"TEST_VAR": "test_value",
			},
			cleanupEnv: []string{"TEST_VAR"},
		},
		{
			name:    "command with spaces",
			input:   "`echo 'hello   world'`",
			want:    "hello   world",
			wantErr: false,
		},
		{
			name:    "invalid command",
			input:   "`nonexistentcommand123`",
			wantErr: true,
		},
		{
			name:    "empty backticks",
			input:   "``",
			want:    "``",
			wantErr: false,
		},
		{
			name:    "command that returns error",
			input:   "`exit 1`",
			wantErr: true,
		},
		{
			name:    "command with pipeline",
			input:   "`echo hello | tr 'a-z' 'A-Z'`",
			want:    "HELLO",
			wantErr: false,
		},
		{
			name:    "multiple lines in output",
			input:   "`printf 'line1\\nline2'`",
			want:    "line1\nline2",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if test should be skipped on current OS
			for _, os := range tt.skipOnOS {
				if runtime.GOOS == os {
					t.Skipf("Skipping test on %s", os)
					return
				}
			}

			// Setup environment if needed
			if tt.setupEnv != nil {
				for k, v := range tt.setupEnv {
					oldValue := os.Getenv(k)
					os.Setenv(k, v)
					defer os.Setenv(k, oldValue)
				}
			}

			// Run test
			got, err := substituteCommands(tt.input)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("substituteCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expect an error, don't check the output
			if tt.wantErr {
				return
			}

			// Compare output
			if got != tt.want {
				t.Errorf("substituteCommands() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSubstituteCommandsEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "only spaces",
			input:   "     ",
			want:    "     ",
			wantErr: false,
		},
		{
			name:    "unmatched backticks",
			input:   "hello `world",
			want:    "hello `world",
			wantErr: false,
		},
		{
			name:    "escaped backticks",
			input:   "hello \\`world\\`",
			want:    "hello \\`world\\`",
			wantErr: false,
		},
		{
			name:    "multiple backticks without command",
			input:   "``````",
			want:    "``````",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := substituteCommands(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("substituteCommands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("substituteCommands() = %q, want %q", got, tt.want)
			}
		})
	}
}
