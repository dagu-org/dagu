// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "hours only",
			input: "6h",
			want:  6 * time.Hour,
		},
		{
			name:  "days and hours",
			input: "2d12h",
			want:  60 * time.Hour,
		},
		{
			name:  "days and minutes",
			input: "1d30m",
			want:  24*time.Hour + 30*time.Minute,
		},
		{
			name:  "minutes only",
			input: "90m",
			want:  90 * time.Minute,
		},
		{
			name:  "days only",
			input: "7d",
			want:  7 * 24 * time.Hour,
		},
		{
			name:  "all units",
			input: "1d2h30m",
			want:  24*time.Hour + 2*time.Hour + 30*time.Minute,
		},
		{
			name:  "minutes only single",
			input: "1m",
			want:  time.Minute,
		},
		{
			name:    "zero hours invalid",
			input:   "0h",
			wantErr: true,
		},
		{
			name:    "zero value invalid",
			input:   "0m",
			wantErr: true,
		},
		{
			name:    "empty string invalid",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing unit invalid",
			input:   "2d12",
			wantErr: true,
		},
		{
			name:    "no number invalid",
			input:   "d",
			wantErr: true,
		},
		{
			name:    "unknown unit",
			input:   "5s",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:  "with leading whitespace",
			input: " 6h",
			want:  6 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
