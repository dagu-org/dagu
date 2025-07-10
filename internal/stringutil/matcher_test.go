package stringutil_test

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/stringutil"
)

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

func TestMatchPattern_LongLines(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name     string
		size     int
		pattern  string
		expected bool
	}{
		{
			name:     "short line matches regex",
			size:     100,
			pattern:  "re:.+",
			expected: true,
		},
		{
			name:     "45KB line matches regex",
			size:     45_000,
			pattern:  "re:.+",
			expected: true,
		},
		{
			name:     "65KB line (over default buffer) matches regex",
			size:     65_000,
			pattern:  "re:.+", 
			expected: true,
		},
		{
			name:     "78KB line (user's case) matches regex",
			size:     78_000,
			pattern:  "re:.+",
			expected: true,
		},
		{
			name:     "100KB line matches regex",
			size:     100_000,
			pattern:  "re:.+",
			expected: true,
		},
		{
			name:     "900KB line (under 1MB limit) matches regex",
			size:     900_000,
			pattern:  "re:.+",
			expected: true,
		},
		{
			name:     "empty string doesn't match .+ regex",
			size:     0,
			pattern:  "re:.+",
			expected: false,
		},
		{
			name:     "long line matches specific pattern",
			size:     78_000,
			pattern:  "re:^x+$",
			expected: true,
		},
		{
			name:     "long line with literal pattern",
			size:     78_000,
			pattern:  "xxx",
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Repeat("x", tt.size)
			result := stringutil.MatchPattern(ctx, content, []string{tt.pattern})
			
			if result != tt.expected {
				t.Errorf("MatchPattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchPattern_MultipleLines(t *testing.T) {
	ctx := context.Background()
	
	// Test with multiple lines where one line is very long
	longLine := strings.Repeat("x", 78_000)
	content := "short line\n" + longLine + "\nlast line"
	
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "matches short line",
			pattern:  "short",
			expected: true,
		},
		{
			name:     "matches long line with regex", 
			pattern:  "re:x{1000,}",
			expected: true,
		},
		{
			name:     "matches last line",
			pattern:  "last",
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringutil.MatchPattern(ctx, content, []string{tt.pattern})
			
			if result != tt.expected {
				t.Errorf("MatchPattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}
