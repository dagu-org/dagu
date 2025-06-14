package cmdutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandReferencesWithSteps(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		stepMap map[string]StepInfo
		want    string
	}{
		{
			name:    "Basic step ID stdout reference",
			input:   "Log file is at ${download.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout:   "/tmp/logs/download.out",
					Stderr:   "/tmp/logs/download.err",
					ExitCode: "0",
				},
			},
			want: "Log file is at /tmp/logs/download.out",
		},
		{
			name:    "Step ID stderr reference",
			input:   "Check errors at ${build.stderr}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"build": {
					Stdout:   "/tmp/logs/build.out",
					Stderr:   "/tmp/logs/build.err",
					ExitCode: "1",
				},
			},
			want: "Check errors at /tmp/logs/build.err",
		},
		{
			name:    "Step ID exit code reference",
			input:   "Build exited with code ${build.exit_code}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"build": {
					Stdout:   "/tmp/logs/build.out",
					Stderr:   "/tmp/logs/build.err",
					ExitCode: "1",
				},
			},
			want: "Build exited with code 1",
		},
		{
			name:    "Step ID outputs reference",
			input:   "Database host is ${config.outputs.db_host}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"config": {
					Stdout:   "/tmp/logs/config.out",
					Stderr:   "/tmp/logs/config.err",
					ExitCode: "0",
					Outputs: map[string]string{
						"db_host": "localhost",
						"db_port": "5432",
					},
				},
			},
			want: "Database host is localhost",
		},
		{
			name:    "Multiple step references",
			input:   "Download log: ${download.stdout}, Build errors: ${build.stderr}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
				"build": {
					Stderr: "/tmp/logs/build.err",
				},
			},
			want: "Download log: /tmp/logs/download.out, Build errors: /tmp/logs/build.err",
		},
		{
			name:    "Unknown step ID leaves as-is",
			input:   "Unknown step: ${unknown.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"known": {
					Stdout: "/tmp/logs/known.out",
				},
			},
			want: "Unknown step: ${unknown.stdout}",
		},
		{
			name:    "Unknown property leaves as-is",
			input:   "Unknown prop: ${download.unknown}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
			},
			want: "Unknown prop: ${download.unknown}",
		},
		{
			name:    "Regular variable takes precedence over step ID",
			input:   "Value: ${download.stdout}",
			dataMap: map[string]string{
				"download": `{"stdout": "from-variable"}`,
			},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
			},
			want: "Value: from-variable",
		},
		{
			name:    "Nested outputs path",
			input:   "Nested value: ${config.outputs.database.host}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"config": {
					Outputs: map[string]string{
						"database": `{"host": "db.example.com", "port": 5432}`,
					},
				},
			},
			want: "Nested value: db.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ExpandReferencesWithSteps(ctx, tt.input, tt.dataMap, tt.stepMap)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalStringWithSteps(t *testing.T) {
	ctx := context.Background()
	
	stepMap := map[string]StepInfo{
		"download": {
			Stdout:   "/var/log/download.stdout",
			Stderr:   "/var/log/download.stderr",
			ExitCode: "0",
		},
		"process": {
			Stdout:   "/var/log/process.stdout",
			Stderr:   "/var/log/process.stderr",
			ExitCode: "1",
			Outputs: map[string]string{
				"result": "success",
				"count":  "42",
			},
		},
	}
	
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "stdout reference",
			input: "cat ${download.stdout}",
			want:  "cat /var/log/download.stdout",
		},
		{
			name:  "stderr reference", 
			input: "tail -20 ${process.stderr}",
			want:  "tail -20 /var/log/process.stderr",
		},
		{
			name:  "exit code reference",
			input: "if [ ${process.exit_code} -ne 0 ]; then echo failed; fi",
			want:  "if [ 1 -ne 0 ]; then echo failed; fi",
		},
		{
			name:  "outputs reference",
			input: "Result was ${process.outputs.result} with count ${process.outputs.count}",
			want:  "Result was success with count 42",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalString(ctx, tt.input, WithStepMap(stepMap))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}