// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ParseDuration parses a duration string supporting m (minutes), h (hours),
// and d (days = 24h) tokens. The grammar is a sum of <int><unit> tokens
// (e.g. "2d12h" = 60h). Rejects empty, missing units, negatives, and zero.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	var total time.Duration
	remaining := s
	tokenCount := 0

	for remaining != "" {
		// Skip leading whitespace within the string
		remaining = strings.TrimLeftFunc(remaining, unicode.IsSpace)
		if remaining == "" {
			break
		}

		// Read the numeric part
		i := 0
		for i < len(remaining) && remaining[i] >= '0' && remaining[i] <= '9' {
			i++
		}

		if i == 0 {
			return 0, fmt.Errorf("invalid duration %q: expected a number", s)
		}

		numStr := remaining[:i]
		remaining = remaining[i:]

		num, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}

		if num < 0 {
			return 0, fmt.Errorf("invalid duration %q: negative values not allowed", s)
		}

		// Read the unit
		if remaining == "" {
			return 0, fmt.Errorf("invalid duration %q: missing unit after %s", s, numStr)
		}

		unit := remaining[0]
		remaining = remaining[1:]

		switch unit {
		case 'm':
			total += time.Duration(num) * time.Minute
		case 'h':
			total += time.Duration(num) * time.Hour
		case 'd':
			total += time.Duration(num) * 24 * time.Hour
		default:
			return 0, fmt.Errorf("invalid duration %q: unknown unit %q", s, string(unit))
		}

		tokenCount++
	}

	if tokenCount == 0 {
		return 0, fmt.Errorf("invalid duration %q: no tokens found", s)
	}

	if total == 0 {
		return 0, fmt.Errorf("invalid duration %q: duration must be positive", s)
	}

	return total, nil
}
