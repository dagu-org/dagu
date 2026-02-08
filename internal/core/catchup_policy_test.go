package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatchupPolicy_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policy CatchupPolicy
		want   string
	}{
		{CatchupPolicyOff, "off"},
		{CatchupPolicyLatest, "latest"},
		{CatchupPolicyAll, "all"},
		{CatchupPolicy(99), "off"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.policy.String())
	}
}

func TestParseCatchupPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    CatchupPolicy
		wantErr bool
	}{
		{"", CatchupPolicyOff, false},
		{"false", CatchupPolicyOff, false},
		{"off", CatchupPolicyOff, false},
		{"latest", CatchupPolicyLatest, false},
		{"all", CatchupPolicyAll, false},
		{"true", CatchupPolicyAll, false},
		{"invalid", CatchupPolicyOff, true},
	}

	for _, tt := range tests {
		got, err := ParseCatchupPolicy(tt.input)
		if tt.wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, tt.want, got)
	}
}
