package grep

import (
	"bufio"
	"bytes"
	"errors"
	"regexp"
	"strings"

	"github.com/samber/lo"
)

var (
	// ErrNoMatch is returned when no match is found.
	ErrNoMatch = errors.New("no matched")
	// ErrEmptyPattern is returned when pattern is empty.
	ErrEmptyPattern = errors.New("empty pattern")
)

// Matcher is the interface for matching lines.
type Matcher interface {
	// Match returns true if the given line matches.
	Match(line string) bool
}

// Options represents grep options.
// If IsRegexp is true, the pattern is treated as a regular expression.
// Before and After are the number of lines before and after the matched line.
type Options struct {
	IsRegexp bool
	Before   int
	After    int
	Matcher  Matcher
}

// Match contains matched line number and line content.
type Match struct {
	Line       string
	LineNumber int
	StartLine  int
}

// Grep reads data and returns lines that match the given pattern.
// If opts is nil, default options will be used.
func Grep(dat []byte, pattern string, opts *Options) ([]*Match, error) {
	if opts == nil {
		opts = new(Options)
	}
	if pattern == "" {
		return nil, ErrEmptyPattern
	}

	matcher, err := getMatcher(pattern, opts)
	if err != nil {
		return nil, err
	}

	lines, matches, err := scanLines(dat, matcher)
	if err != nil {
		return nil, err
	}

	return buildMatches(lines, matches, opts), nil
}

// getMatcher returns a matcher based on the pattern and options.
func getMatcher(pattern string, opts *Options) (Matcher, error) {
	if opts.Matcher != nil {
		return opts.Matcher, nil
	}
	return defaultMatcher(pattern, opts)
}

// scanLines scans through data and returns lines and their matched indices.
func scanLines(dat []byte, matcher Matcher) ([]string, []int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(dat))
	var lines []string
	var matches []int
	var idx int

	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if matcher.Match(line) {
			matches = append(matches, idx)
		}
		idx++
	}

	if len(matches) == 0 {
		return nil, nil, ErrNoMatch
	}
	return lines, matches, scanner.Err()
}

// buildMatches constructs Match objects from matched line indices.
func buildMatches(lines []string, matches []int, opts *Options) []*Match {
	var ret []*Match

	for _, m := range matches {
		low := lo.Max([]int{0, m - opts.Before})
		high := lo.Min([]int{len(lines), m + opts.After + 1})
		matchText := strings.Join(lines[low:high], "\n")

		ret = append(ret, &Match{
			StartLine:  low + 1,
			LineNumber: m + 1,
			Line:       matchText,
		})
	}
	return ret
}

func defaultMatcher(pattern string, opts *Options) (Matcher, error) {
	if opts.IsRegexp {
		reg, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return &regexpMatcher{
			Regexp: reg,
		}, nil
	}
	return &simpleMatcher{
		Pattern: pattern,
	}, nil
}

var _ Matcher = (*regexpMatcher)(nil)

type regexpMatcher struct {
	Regexp *regexp.Regexp
}

func (rm *regexpMatcher) Match(line string) bool {
	return rm.Regexp.MatchString(line)
}

var _ Matcher = (*simpleMatcher)(nil)

type simpleMatcher struct {
	Pattern string
}

func (rm *simpleMatcher) Match(line string) bool {
	return strings.Contains(line, rm.Pattern)
}
