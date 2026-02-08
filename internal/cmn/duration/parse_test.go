// Copyright 2025 The Dagu Authors
//
// Licensed under the GNU Affero General Public License v3.0.
// You may obtain a copy of the License at https://www.gnu.org/licenses/agpl-3.0.html

package duration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"days", "2d", 48 * time.Hour, false},
		{"days and hours", "2d12h", 60 * time.Hour, false},
		{"days hours minutes", "1d2h30m", 26*time.Hour + 30*time.Minute, false},
		{"hours only", "6h", 6 * time.Hour, false},
		{"minutes only", "90m", 90 * time.Minute, false},
		{"seconds", "30s", 30 * time.Second, false},
		{"complex", "1d30m", 24*time.Hour + 30*time.Minute, false},
		{"zero", "0s", 0, false},
		{"empty string", "", 0, true},
		{"negative", "-1h", 0, true},
		{"invalid", "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
