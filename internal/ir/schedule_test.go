// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package ir

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckMisleadingStepValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		expr        string
		wantWarning bool
		contains    []string
	}{
		{
			name:        "minute step 33 warns",
			expr:        "*/33 * * * *",
			wantWarning: true,
			contains:    []string{"*/33", "minute field", "not every 33 minutes"},
		},
		{
			name:        "minute step 5 is valid",
			expr:        "*/5 * * * *",
			wantWarning: false,
		},
		{
			name:        "minute step 15 is valid",
			expr:        "*/15 * * * *",
			wantWarning: false,
		},
		{
			name:        "minute step 7 warns",
			expr:        "*/7 * * * *",
			wantWarning: true,
			contains:    []string{"*/7", "minute field", "not every 7 minutes"},
		},
		{
			name:        "hour step 5 is valid",
			expr:        "0 */5 * * *",
			wantWarning: false,
		},
		{
			name:        "hour step 7 warns",
			expr:        "0 */7 * * *",
			wantWarning: true,
			contains:    []string{"*/7", "hour field", "not every 7 hours"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			warnings := checkMisleadingStepValues(tt.expr)
			if tt.wantWarning {
				require.NotEmpty(t, warnings)
				for _, s := range tt.contains {
					require.Contains(t, warnings[0], s)
				}
				return
			}

			require.Empty(t, warnings)
		})
	}
}

func TestCronDescriptors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 6, 10, 25, 0, 0, time.UTC)
	cases := []struct {
		descriptor string
		expression string
		next       string
	}{
		{"@hourly", "0 * * * *", "2026-09-06T11:00:00Z"},
		{"@daily", "0 0 * * *", "2026-09-07T00:00:00Z"},
		{"@weekly", "0 0 * * 0", "2026-09-13T00:00:00Z"},
		{"@monthly", "0 0 1 * *", "2026-10-01T00:00:00Z"},
		{"@yearly", "0 0 1 1 *", "2027-01-01T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.descriptor, func(t *testing.T) {
			t.Parallel()

			schedule, err := NewCronSchedule(tc.descriptor)
			require.NoError(t, err)
			require.Equal(t, tc.expression, schedule.Expression)
			require.Equal(t, tc.next, schedule.Next(now).Format(time.RFC3339))
		})
	}
}
