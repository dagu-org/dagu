package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveStepProperty(t *testing.T) {
	tests := []struct {
		name    string
		step    string
		path    string
		stepMap map[string]StepInfo
		wantOK  bool
	}{
		{
			name:    "EmptyStderr",
			step:    "step1",
			path:    ".stderr",
			stepMap: map[string]StepInfo{"step1": {Stdout: "out", Stderr: "", ExitCode: "0"}},
			wantOK:  false,
		},
		{
			name:    "UnknownProperty",
			step:    "step1",
			path:    ".unknown_prop",
			stepMap: map[string]StepInfo{"step1": {Stdout: "out", ExitCode: "0"}},
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := resolveStepProperty(context.Background(), tt.step, tt.path, tt.stepMap)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestParseStepReference(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		wantProp      string
		wantHasStart  bool
		wantStart     int
		wantHasLength bool
		wantLength    int
		wantErr       bool
		errMsg        string
	}{
		{
			name:     "NoSlice",
			path:     ".stdout",
			wantProp: ".stdout",
		},
		{
			name:         "WithStartOnly",
			path:         ".stdout:3",
			wantProp:     ".stdout",
			wantHasStart: true,
			wantStart:    3,
		},
		{
			name:          "WithStartAndLength",
			path:          ".stdout:3:5",
			wantProp:      ".stdout",
			wantHasStart:  true,
			wantStart:     3,
			wantHasLength: true,
			wantLength:    5,
		},
		{
			name:    "EmptySliceSpec",
			path:    ".stdout:",
			wantErr: true,
			errMsg:  "slice specification missing values",
		},
		{
			name:    "TooManyParts",
			path:    ".stdout:1:2:3",
			wantErr: true,
			errMsg:  "too many slice sections",
		},
		{
			name:    "EmptyOffset",
			path:    ".stdout::5",
			wantErr: true,
			errMsg:  "slice offset is required",
		},
		{
			name:    "InvalidOffset",
			path:    ".stdout:abc",
			wantErr: true,
			errMsg:  "invalid slice offset",
		},
		{
			name:    "NegativeOffset",
			path:    ".stdout:-1",
			wantErr: true,
			errMsg:  "slice offset must be non-negative",
		},
		{
			name:    "InvalidLength",
			path:    ".stdout:0:xyz",
			wantErr: true,
			errMsg:  "invalid slice length",
		},
		{
			name:    "NegativeLength",
			path:    ".stdout:0:-5",
			wantErr: true,
			errMsg:  "slice length must be non-negative",
		},
		{
			name:          "ZeroStart",
			path:          ".exit_code:0:10",
			wantProp:      ".exit_code",
			wantHasStart:  true,
			wantStart:     0,
			wantHasLength: true,
			wantLength:    10,
		},
		{
			name:          "EmptyLengthPart",
			path:          ".stdout:5:",
			wantProp:      ".stdout",
			wantHasStart:  true,
			wantStart:     5,
			wantHasLength: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, spec, err := parseStepReference(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProp, prop)
			assert.Equal(t, tt.wantHasStart, spec.hasStart)
			if tt.wantHasStart {
				assert.Equal(t, tt.wantStart, spec.start)
			}
			assert.Equal(t, tt.wantHasLength, spec.hasLength)
			if tt.wantHasLength {
				assert.Equal(t, tt.wantLength, spec.length)
			}
		})
	}
}

func TestApplyStepSlice(t *testing.T) {
	tests := []struct {
		name  string
		value string
		spec  stepSliceSpec
		want  string
	}{
		{
			name:  "NoSlice",
			value: "hello",
			spec:  stepSliceSpec{},
			want:  "hello",
		},
		{
			name:  "StartOnly",
			value: "hello world",
			spec:  stepSliceSpec{hasStart: true, start: 6},
			want:  "world",
		},
		{
			name:  "StartAndLength",
			value: "hello world",
			spec:  stepSliceSpec{hasStart: true, start: 0, hasLength: true, length: 5},
			want:  "hello",
		},
		{
			name:  "StartBeyondLength",
			value: "short",
			spec:  stepSliceSpec{hasStart: true, start: 100},
			want:  "",
		},
		{
			name:  "LengthExceedsRemainder",
			value: "hello",
			spec:  stepSliceSpec{hasStart: true, start: 3, hasLength: true, length: 100},
			want:  "lo",
		},
		{
			name:  "UnicodeChars",
			value: "日本語テスト",
			spec:  stepSliceSpec{hasStart: true, start: 0, hasLength: true, length: 3},
			want:  "日本語",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyStepSlice(tt.value, tt.spec)
			assert.Equal(t, tt.want, got)
		})
	}
}
