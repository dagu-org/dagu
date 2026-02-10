// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOverlapPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    OverlapPolicy
		wantErr bool
	}{
		{
			name:  "skip",
			input: "skip",
			want:  OverlapPolicySkip,
		},
		{
			name:  "all",
			input: "all",
			want:  OverlapPolicyAll,
		},
		{
			name:  "empty defaults to skip",
			input: "",
			want:  OverlapPolicySkip,
		},
		{
			name:    "bogus is invalid",
			input:   "bogus",
			wantErr: true,
		},
		{
			name:    "invalid value",
			input:   "queue",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseOverlapPolicy(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
