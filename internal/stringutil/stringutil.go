package stringutil

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
)

const (
	legacyTimeFormat = "2006-01-02 15:04:05"
	timeEmpty        = "-"
	rePrefix         = "re:"
)

type PairString string

func NewPairString(key, value string) PairString {
	return PairString(key + "=" + value)
}

func (kv PairString) Key() string {
	parts := strings.SplitN(string(kv), "=", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func (kv PairString) Value() string {
	parts := strings.SplitN(string(kv), "=", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (kv PairString) Bool() bool {
	v, err := strconv.ParseBool(kv.Value())
	if err != nil {
		return false
	}
	return v
}

func (kv PairString) String() string {
	return string(kv)
}

// MatchOption represents an option for pattern matching
type MatchOption func(*matchOptions)

type matchOptions struct {
	exactMatch bool
}

// WithExactMatch configures the matcher to use exact string matching for literal patterns
func WithExactMatch() MatchOption {
	return func(o *matchOptions) {
		o.exactMatch = true
	}
}

// MatchPattern matches content against patterns using either literal or regex matching.
// For files or large content, use MatchPatternScanner instead.
func MatchPattern(ctx context.Context, content string, patterns []string, opts ...MatchOption) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	return MatchPatternScanner(ctx, scanner, patterns, opts...)
}

func MatchPatternScanner(ctx context.Context, scanner *bufio.Scanner, patterns []string, opts ...MatchOption) bool {
	if len(patterns) == 0 {
		return false
	}

	// Apply options
	options := &matchOptions{}
	for _, opt := range opts {
		opt(options)
	}

	var regexps []*regexp.Regexp
	var literalPatterns []string

	// Compile regex patterns first
	for _, pattern := range patterns {
		switch {
		case strings.HasPrefix(pattern, rePrefix):
			re, err := regexp.Compile(strings.TrimPrefix(pattern, rePrefix))
			if err != nil {
				logger.Error(ctx, "invalid regexp pattern", "pattern", pattern, "err", err)
				continue
			}
			regexps = append(regexps, re)
		case strings.HasPrefix(pattern, rePrefix):
			re, err := regexp.Compile(strings.TrimPrefix(pattern, rePrefix))
			if err != nil {
				logger.Error(ctx, "invalid regexp pattern", "pattern", pattern, "err", err)
				continue
			}
			regexps = append(regexps, re)
		default:
			literalPatterns = append(literalPatterns, pattern)
		}
	}

	// Special case: if scanner is empty and we're looking for empty string
	if !scanner.Scan() {
		// Check for empty string patterns
		for _, p := range literalPatterns {
			if p == "" {
				return true
			}
		}
		// Check regex patterns against empty string
		for _, re := range regexps {
			if re.MatchString("") {
				return true
			}
		}
		return false
	}

	// Process first line (already read by scanner.Scan() above)
	line := scanner.Text()
	if matchLine(line, literalPatterns, regexps, options) {
		return true
	}

	// Process remaining lines
	for scanner.Scan() {
		if matchLine(scanner.Text(), literalPatterns, regexps, options) {
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error(ctx, "scanner error", "err", err)
	}

	return false
}

// matchLine checks if a single line matches any of the patterns
func matchLine(line string, literalPatterns []string, regexps []*regexp.Regexp, opts *matchOptions) bool {
	// Check literal patterns
	for _, p := range literalPatterns {
		if opts.exactMatch {
			if line == p {
				return true
			}
		} else {
			if strings.Contains(line, p) {
				return true
			}
		}
	}

	// Check regex patterns
	for _, re := range regexps {
		if re.MatchString(line) {
			return true
		}
	}

	return false
}

// FormatTime returns formatted time.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return timeEmpty
	}

	return t.Format(time.RFC3339)
}

// ParseTime parses time string.
func ParseTime(val string) (time.Time, error) {
	if val == timeEmpty {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation(time.RFC3339, val, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation(legacyTimeFormat, val, time.Local)
}

// TruncString TurnString returns truncated string.
func TruncString(val string, max int) string {
	if len(val) > max {
		return val[:max]
	}
	return val
}
