package grep

import (
	"bufio"
	"bytes"
	"errors"
	"os"
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

// Options represents grep options.
// If IsRegexp is true, the pattern is treated as a regular expression.
// Before and After are the number of lines before and after the matched line.
type Options struct {
	IsRegexp bool
	Before   int
	After    int
}

// Match contains matched line number and line content.
type Match struct {
	Line       string
	LineNumber int
	StartLine  int
}

// Grep read file and return matched lines.
// If opts is nil, default options will be used.
// The result is a map, key is line number, value is line content.
func Grep(file string, pattern string, opts *Options) ([]*Match, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = &Options{}
	}
	if pattern == "" {
		return nil, ErrEmptyPattern
	}
	var reg *regexp.Regexp = nil
	if opts.IsRegexp {
		if reg, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	ret := []*Match{}
	lines := []string{}
	matched := []int{}
	i := 0
	for scanner.Scan() {
		t := scanner.Text()
		lines = append(lines, t)
		flag := false
		if opts.IsRegexp && reg.MatchString(t) {
			flag = true
		} else if strings.Contains(t, pattern) {
			flag = true
		}
		if flag {
			matched = append(matched, i)
		}
		i++
	}
	if len(matched) == 0 {
		return nil, ErrNoMatch
	}
	for _, m := range matched {
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
