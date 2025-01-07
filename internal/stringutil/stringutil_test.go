package stringutil_test

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/stretchr/testify/require"
)

func Test_MustGetUserHomeDir(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		err := os.Setenv("HOME", "/test")
		if err != nil {
			t.Fatal(err)
		}
		hd := fileutil.MustGetUserHomeDir()
		require.Equal(t, "/test", hd)
	})
}

func Test_MustGetwd(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		wd, _ := os.Getwd()
		require.Equal(t, fileutil.MustGetwd(), wd)
	})
}

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

func TestMatchPattern(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		val      string
		patterns []string
		want     bool
	}{
		{
			name:     "empty patterns",
			val:      "test",
			patterns: []string{},
			want:     false,
		},
		{
			name:     "exact literal match",
			val:      "test",
			patterns: []string{"test"},
			want:     true,
		},
		{
			name:     "no literal match",
			val:      "test",
			patterns: []string{"foo", "bar"},
			want:     false,
		},
		{
			name:     "simple regex match",
			val:      "test123",
			patterns: []string{"re:test\\d+"},
			want:     true,
		},
		{
			name:     "no regex match",
			val:      "test",
			patterns: []string{"re:\\d+"},
			want:     false,
		},
		{
			name:     "invalid regex pattern",
			val:      "test",
			patterns: []string{"re:["},
			want:     false,
		},
		{
			name:     "mixed patterns with literal match",
			val:      "test123",
			patterns: []string{"re:\\d+", "test123", "foo"},
			want:     true,
		},
		{
			name:     "mixed patterns with regex match",
			val:      "test123",
			patterns: []string{"foo", "bar", "re:test\\d+"},
			want:     true,
		},
		{
			name:     "case sensitive literal match",
			val:      "Test",
			patterns: []string{"test"},
			want:     false,
		},
		{
			name:     "case sensitive regex match",
			val:      "Test123",
			patterns: []string{"re:test\\d+"},
			want:     false,
		},
		{
			name:     "case insensitive regex match",
			val:      "Test123",
			patterns: []string{"re:(?i)test\\d+"},
			want:     true,
		},
		{
			name:     "empty value",
			val:      "",
			patterns: []string{"re:.*", ""},
			want:     true,
		},
		{
			name:     "special characters in literal match",
			val:      "test.123",
			patterns: []string{"test.123"},
			want:     true,
		},
		{
			name:     "special characters in regex match",
			val:      "test.123",
			patterns: []string{"re:test\\.\\d+"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringutil.MatchPattern(ctx, tt.val, tt.patterns)
			if got != tt.want {
				t.Errorf("MatchString() = %v, want %v", got, tt.want)
			}

			// Test scanner version with the same input
			scanner := bufio.NewScanner(strings.NewReader(tt.val))
			got = stringutil.MatchPatternScanner(ctx, scanner, tt.patterns)
			if got != tt.want {
				t.Errorf("MatchPatternScanner() = %v, want %v", got, tt.want)
			}
		})
	}
}
