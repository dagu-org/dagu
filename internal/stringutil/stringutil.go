package stringutil

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
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

// IsJSONArray checks if the given string is a valid JSON array.
// This is useful for determining if parallel input should be parsed as JSON or space-separated.
func IsJSONArray(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}

	// Quick check for brackets
	if s[0] != '[' || s[len(s)-1] != ']' {
		return false
	}

	// Use json.Valid for accurate validation
	return json.Valid([]byte(s))
}

// KebabToCamel converts a kebab-case string to camelCase.
func KebabToCamel(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) == 0 {
		return ""
	}

	// Find the first non-empty part for the initial word
	result := ""
	startIdx := 0
	for i := 0; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result = parts[i]
			startIdx = i + 1
			break
		}
	}

	// Capitalize remaining parts
	for i := startIdx; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}

	return result
}

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandomString(n int) string {
	b := make([]byte, n)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
