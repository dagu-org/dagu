package utils

import (
	"io/ioutil"
	"jobctl/internal/constants"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func DefaultEnv() map[string]string {
	return map[string]string{
		"PATH": "${PATH}",
	}
}

// MustGetUserHomeDir returns current working directory.
// Panics is os.UserHomeDir() returns error
func MustGetUserHomeDir() string {
	hd, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return hd
}

// MustGetwd returns current working directory.
// Panics is os.Getwd() returns error
func MustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return wd
}

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return constants.TimeEmpty
	} else {
		return t.Format(constants.TimeFormat)
	}
}

func ParseTime(val string) (time.Time, error) {
	if val == constants.TimeEmpty {
		return time.Time{}, nil
	}
	ret, err := time.ParseInLocation(constants.TimeFormat, val, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	return ret, nil
}

func FormatDuration(t time.Duration, defaultVal string) string {
	if t == 0 {
		return defaultVal
	} else {
		return t.String()
	}
}

func SplitCommand(cmd string) (program string, args []string) {
	vals := strings.SplitN(os.ExpandEnv(cmd), " ", 2)
	if len(vals) > 1 {
		return vals[0], strings.Split(vals[1], " ")
	}
	return vals[0], []string{}
}

func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func OpenOrCreateFile(file string) (*os.File, error) {
	if FileExists(file) {
		return OpenFile(file)
	}
	return CreateFile(file)
}

func OpenFile(file string) (*os.File, error) {
	outfile, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}
	return outfile, nil
}

func CreateFile(file string) (*os.File, error) {
	outfile, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	return outfile, nil
}

// https://github.com/sindresorhus/filename-reserved-regex/blob/master/index.js
var (
	filenameReservedRegex             = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	filenameReservedWindowsNamesRegex = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`)
)

func ValidFilename(str, replacement string) string {
	s := filenameReservedRegex.ReplaceAllString(str, replacement)
	s = filenameReservedWindowsNamesRegex.ReplaceAllString(s, replacement)
	return strings.ReplaceAll(s, " ", replacement)
}

func ParseVariable(value string) (string, error) {
	val, err := ParseCommand(os.ExpandEnv(value))
	if err != nil {
		return "", err
	}
	return val, nil
}

var tickerMatcher = regexp.MustCompile("`[^`]+`")

func ParseCommand(value string) (string, error) {
	matches := tickerMatcher.FindAllString(strings.TrimSpace(value), -1)
	if matches == nil {
		return value, nil
	}
	ret := value
	for i := 0; i < len(matches); i++ {
		command := matches[i]
		str := strings.ReplaceAll(command, "`", "")
		prog, args := SplitCommand(str)
		out, err := exec.Command(prog, args...).Output()
		if err != nil {
			return "", err
		}
		ret = strings.ReplaceAll(ret, command, strings.TrimSpace(string(out[:])))

	}
	return ret, nil
}

func MustTempDir(pattern string) string {
	t, err := ioutil.TempDir("", pattern)
	if err != nil {
		panic(err)
	}
	return t
}
