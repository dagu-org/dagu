package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetTimezone(t *testing.T) {
	t.Parallel()
	t.Run("ValidTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Global{TZ: "America/New_York"}
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
		g := &Global{TZ: "UTC"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "UTC", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 0, g.TzOffsetInSec)
	})

	t.Run("AsiaTokyoTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Global{TZ: "Asia/Tokyo"}
		err := setTimezone(g)
		require.NoError(t, err)

		assert.Equal(t, "Asia/Tokyo", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 9*3600, g.TzOffsetInSec) // Tokyo is UTC+9
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		t.Parallel()
		g := &Global{TZ: "Invalid/Timezone"}
		err := setTimezone(g)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	t.Run("EmptyTimezoneUsesLocal", func(t *testing.T) {
		t.Parallel()
		g := &Global{TZ: ""}
		err := setTimezone(g)
		require.NoError(t, err)

		// Should set TZ to UTC or UTC+X format
		assert.NotEmpty(t, g.TZ)
		assert.NotNil(t, g.Location)
	})
}
