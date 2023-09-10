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

// Grep read file and return matched lines.
// If opts is nil, default options will be used.
func Grep(dat []byte, pattern string, opts *Options) ([]*Match, error) {
	if opts == nil {
		opts = &Options{}
	}
	if pattern == "" {
		return nil, ErrEmptyPattern
	}
	matcher := opts.Matcher
	if matcher == nil {
		var err error
		if matcher, err = defaultMatcher(pattern, opts); err != nil {
			return nil, err
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(dat))
	ret := make([]*Match, 0)
	lines := make([]string, 0)
	matches := make([]int, 0)
	i := 0
	for scanner.Scan() {
		t := scanner.Text()
		lines = append(lines, t)
		if matcher.Match(t) {
			matches = append(matches, i)
		}
		i++
	}
	if len(matches) == 0 {
		return nil, ErrNoMatch
	}
	for _, m := range matches {
		l := lo.Max([]int{0, m - opts.Before})
		h := lo.Min([]int{len(lines), m + opts.After + 1})
		s := strings.Join(lines[l:h], "\n")
		ret = append(ret, &Match{
			StartLine:  l + 1,
			LineNumber: m + 1,
			Line:       s,
		})
	}
	return ret, nil
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

type regexpMatcher struct {
	Regexp *regexp.Regexp
}

var _ Matcher = (*regexpMatcher)(nil)

func (rm *regexpMatcher) Match(line string) bool {
	return rm.Regexp.MatchString(line)
}

type simpleMatcher struct {
	Pattern string
}

var _ Matcher = (*simpleMatcher)(nil)

func (rm *simpleMatcher) Match(line string) bool {
	return strings.Contains(line, rm.Pattern)
}
