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
	ErrNoMatch      = errors.New("no matched")
	ErrEmptyPattern = errors.New("empty pattern")
)

type Options struct {
	Regexp bool
	Before int
	After  int
}

func Grep(file string, pattern string, opts *Options) (map[int]string, error) {
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
	if opts.Regexp {
		if reg, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	ret := map[int]string{}
	lines := []string{}
	matched := []int{}
	i := 0
	for scanner.Scan() {
		t := scanner.Text()
		lines = append(lines, t)
		flag := false
		if opts.Regexp && reg.MatchString(t) {
			flag = true
		} else if strings.Contains(t, pattern) {
			flag = true
		}
		if flag {
			matched = append(matched, i)
		}
		i++
	}
	for _, m := range matched {
		l := lo.Max([]int{0, m - opts.Before})
		h := lo.Min([]int{len(lines), m + opts.After + 1})
		s := strings.Join(lines[l:h], "\n")
		ret[m+1] = s
	}
	return ret, nil
}
