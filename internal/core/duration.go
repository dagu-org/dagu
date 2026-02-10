package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var reDays = regexp.MustCompile(`(\d+)d`)

// ParseDuration extends time.ParseDuration with support for "d" (days = 24h).
// Rejects empty strings and zero/negative durations.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	s = reDays.ReplaceAllStringFunc(s, func(v string) string {
		n, _ := strconv.Atoi(strings.TrimSuffix(v, "d"))
		return strconv.Itoa(n*24) + "h"
	})
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return d, nil
}
