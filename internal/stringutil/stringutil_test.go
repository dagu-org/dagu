package stringutil_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/stretchr/testify/require"
)

func Test_FormatTime(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		tm := time.Date(2022, 2, 1, 2, 2, 2, 0, time.UTC)
		formatted := stringutil.FormatTime(tm)
		require.Equal(t, "2022-02-01T02:02:02Z", formatted)

		parsed, err := stringutil.ParseTime(formatted)
		require.NoError(t, err)
		require.Equal(t, tm, parsed)

		// Test empty time
		require.Equal(t, "-", stringutil.FormatTime(time.Time{}))
		parsed, err = stringutil.ParseTime("-")
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
	t.Run("Empty", func(t *testing.T) {
		// Test empty time
		require.Equal(t, "-", stringutil.FormatTime(time.Time{}))
	})
}

func Test_ParseTime(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		parsed, err := stringutil.ParseTime("2022-02-01T02:02:02Z")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.UTC), parsed)
	})
	t.Run("Valid_Legacy", func(t *testing.T) {
		parsed, err := stringutil.ParseTime("2022-02-01 02:02:02")
		require.NoError(t, err)
		require.Equal(t, time.Date(2022, 2, 1, 2, 2, 2, 0, time.Now().Location()), parsed)
	})
	t.Run("Empty", func(t *testing.T) {
		parsed, err := stringutil.ParseTime("-")
		require.NoError(t, err)
		require.Equal(t, time.Time{}, parsed)
	})
}

func TestTruncString(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		// Test empty string
		require.Equal(t, "", stringutil.TruncString("", 8))
		// Test string with length less than limit
		require.Equal(t, "1234567", stringutil.TruncString("1234567", 8))
		// Test string with length equal to limit
		require.Equal(t, "12345678", stringutil.TruncString("123456789", 8))
	})
}
