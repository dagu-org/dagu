// Copyright 2025 The Dagu Authors
//
// Licensed under the GNU Affero General Public License v3.0.
// You may obtain a copy of the License at https://www.gnu.org/licenses/agpl-3.0.html

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMisfirePolicy_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policy MisfirePolicy
		want   string
	}{
		{MisfirePolicyIgnore, "ignore"},
		{MisfirePolicyRunOnce, "runOnce"},
		{MisfirePolicyRunLatest, "runLatest"},
		{MisfirePolicyRunAll, "runAll"},
		{MisfirePolicy(99), "ignore"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.policy.String())
	}
}

func TestParseMisfirePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    MisfirePolicy
		wantErr bool
	}{
		{"", MisfirePolicyIgnore, false},
		{"ignore", MisfirePolicyIgnore, false},
		{"runOnce", MisfirePolicyRunOnce, false},
		{"runLatest", MisfirePolicyRunLatest, false},
		{"runAll", MisfirePolicyRunAll, false},
		{"invalid", MisfirePolicyIgnore, true},
	}

	for _, tt := range tests {
		got, err := ParseMisfirePolicy(tt.input)
		if tt.wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, tt.want, got)
	}
}

func TestMisfirePolicy_RoundTrip(t *testing.T) {
	t.Parallel()

	policies := []MisfirePolicy{
		MisfirePolicyIgnore,
		MisfirePolicyRunOnce,
		MisfirePolicyRunLatest,
		MisfirePolicyRunAll,
	}

	for _, p := range policies {
		parsed, err := ParseMisfirePolicy(p.String())
		require.NoError(t, err)
		assert.Equal(t, p, parsed)
	}
}
