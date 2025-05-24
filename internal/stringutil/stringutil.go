package stringutil

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	legacyTimeFormat = "2006-01-02 15:04:05"
)

// FormatTime returns formatted time.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(time.RFC3339)
}

// ParseTime parses time string.
func ParseTime(val string) (time.Time, error) {
	if val == "" || val == "-" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation(time.RFC3339, val, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation(legacyTimeFormat, val, time.Local)
}

// TruncString returns truncated string.
func TruncString(val string, max int) string {
	if len(val) > max {
		return val[:max]
	}
	return val
}

// ParseBool parses a boolean value from the given input.
func ParseBool(_ context.Context, value any) (bool, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseBool(v)
	case bool:
		return v, nil
	default:
		return false, fmt.Errorf("unsupported type %T for bool (value: %+v)", value, value)
	}
}

// RemoveQuotes removes leading and trailing double quotes from a string if present,
// and unescapes the content using strconv.Unquote.
func RemoveQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		unquoted, err := strconv.Unquote(s)
		if err == nil {
			return unquoted
		}
		// If unquoting fails (e.g., malformed string literal),
		// fall back to returning the original string.
	}
	return s
}
