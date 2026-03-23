// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetTimezone(t *testing.T) {
	t.Parallel()
	t.Run("ValidTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Core{TZ: "America/New_York"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "America/New_York", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, "America/New_York", g.Location.String())
		// New York is UTC-5 or UTC-4 depending on DST
		assert.NotEqual(t, 0, g.TzOffsetInSec)
	})

	t.Run("UTCTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Core{TZ: "UTC"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "UTC", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 0, g.TzOffsetInSec)
	})

	t.Run("AsiaTokyoTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Core{TZ: "Asia/Tokyo"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "Asia/Tokyo", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 9*3600, g.TzOffsetInSec) // Tokyo is UTC+9
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Core{TZ: "Invalid/Timezone"}
		err := setTimezone(g)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	t.Run("EmptyTimezoneUsesLocal", func(t *testing.T) {
		t.Parallel()
		g := &Core{TZ: ""}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.NotEmpty(t, g.TZ)
		assert.NotNil(t, g.Location)

		// The TZ value must be loadable so child processes get correct local time.
		if g.TZ != "UTC" {
			_, loadErr := time.LoadLocation(g.TZ)
			assert.NoError(t, loadErr, "cfg.TZ %q should be a valid timezone", g.TZ)
		}
	})
}
