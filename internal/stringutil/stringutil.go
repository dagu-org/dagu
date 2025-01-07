package stringutil

import (
	"bufio"
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
)

const (
	legacyTimeFormat = "2006-01-02 15:04:05"
	timeEmpty        = "-"
	regexpPrefix     = "regexp:"
)

// MatchPattern matches content against patterns using either literal or regex matching.
// For files or large content, use MatchPatternScanner instead.
func MatchPattern(ctx context.Context, content string, patterns []string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	return MatchPatternScanner(ctx, scanner, patterns)
}

func MatchPatternScanner(ctx context.Context, scanner *bufio.Scanner, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	var regexps []*regexp.Regexp
	var literalPatterns []string

	// Compile regex patterns first
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, regexpPrefix) {
			re, err := regexp.Compile(strings.TrimPrefix(pattern, regexpPrefix))
			if err != nil {
				logger.Error(ctx, "invalid regexp pattern", "pattern", pattern, "err", err)
				continue
			}
			regexps = append(regexps, re)
		} else {
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
	if matchLine(line, literalPatterns, regexps) {
		return true
	}

	// Process remaining lines
	for scanner.Scan() {
		if matchLine(scanner.Text(), literalPatterns, regexps) {
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error(ctx, "scanner error", "err", err)
	}

	return false
}

// matchLine checks if a single line matches any of the patterns
func matchLine(line string, literalPatterns []string, regexps []*regexp.Regexp) bool {
	// Check literal patterns
	for _, p := range literalPatterns {
		if strings.Contains(line, p) {
			return true
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
