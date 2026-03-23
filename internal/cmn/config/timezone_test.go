// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveTimezoneState(t *testing.T) {
	t.Helper()

	oldLocal := time.Local
	oldTZ, hadTZ := os.LookupEnv("TZ")

	t.Cleanup(func() {
		time.Local = oldLocal
		if hadTZ {
			if err := os.Setenv("TZ", oldTZ); err != nil {
				t.Fatalf("restore TZ: %v", err)
			}
			return
		}
		if err := os.Unsetenv("TZ"); err != nil {
			t.Fatalf("unset TZ: %v", err)
		}
	})
}

func TestSetTimezone(t *testing.T) {
	t.Run("ValidTimezone", func(t *testing.T) {
		preserveTimezoneState(t)
		g := &Core{TZ: "America/New_York"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "America/New_York", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, "America/New_York", g.Location.String())
		assert.Equal(t, "America/New_York", os.Getenv("TZ"))
		// New York is UTC-5 or UTC-4 depending on DST
		assert.NotEqual(t, 0, g.TzOffsetInSec)
	})

	t.Run("UTCTimezone", func(t *testing.T) {
		preserveTimezoneState(t)
		g := &Core{TZ: "UTC"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "UTC", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 0, g.TzOffsetInSec)
		assert.Equal(t, "UTC", os.Getenv("TZ"))
	})

	t.Run("AsiaTokyoTimezone", func(t *testing.T) {
		preserveTimezoneState(t)
		g := &Core{TZ: "Asia/Tokyo"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "Asia/Tokyo", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 9*3600, g.TzOffsetInSec) // Tokyo is UTC+9
		assert.Equal(t, "Asia/Tokyo", os.Getenv("TZ"))
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		preserveTimezoneState(t)
		g := &Core{TZ: "Invalid/Timezone"}
		err := setTimezone(g)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	t.Run("EmptyTimezoneUsesLegacyLocalDisplay", func(t *testing.T) {
		preserveTimezoneState(t)
		time.Local = time.FixedZone("JST", 9*3600)
		require.NoError(t, os.Unsetenv("TZ"))

		g := &Core{TZ: ""}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "UTC+9", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Same(t, time.Local, g.Location)
		assert.Equal(t, 9*3600, g.TzOffsetInSec)

		_, exists := os.LookupEnv("TZ")
		assert.False(t, exists)
	})
}
