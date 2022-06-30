package utils

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yohamta/dagu/internal/constants"
)

// DefaultEnv returns default value of environment variable.
func DefaultEnv() map[string]string {
	return map[string]string{
		"PATH": "${PATH}",
		"HOME": "${HOME}",
	}
}

// MustGetUserHomeDir returns current working directory.
// Panics is os.UserHomeDir() returns error
func MustGetUserHomeDir() string {
	hd, _ := os.UserHomeDir()
	return hd
}

// MustGetwd returns current working directory.
// Panics is os.Getwd() returns error
func MustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

// FormatTime returns formatted time.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return constants.TimeEmpty
	} else {
		return t.Format(constants.TimeFormat)
	}
}

// ParseTime parses time string.
func ParseTime(val string) (time.Time, error) {
	if val == constants.TimeEmpty {
		return time.Time{}, nil
	}
	return time.ParseInLocation(constants.TimeFormat, val, time.Local)
}

// FormatDuration returns formatted duration.
func FormatDuration(t time.Duration, defaultVal string) string {
	if t == 0 {
		return defaultVal
	} else {
		return t.String()
	}
}

// SplitCommand splits command string to program and arguments.
func SplitCommand(cmd string) (program string, args []string) {
	vals := strings.SplitN(cmd, " ", 2)
	if len(vals) > 1 {
		return vals[0], strings.Split(vals[1], " ")
	}
	return vals[0], []string{}
}

// FileExists returns true if file exists.
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

// OpenOrCreateFile opens file or creates it if it doesn't exist.
func OpenOrCreateFile(file string) (*os.File, error) {
	if FileExists(file) {
		return OpenFile(file)
	}
	return CreateFile(file)
}

// OpenFile opens file.
func OpenFile(file string) (*os.File, error) {
	outfile, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}
	return outfile, nil
}

// CreateFile creates file.
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

// ValidFilename returns true if filename is valid.
func ValidFilename(str, replacement string) string {
	s := filenameReservedRegex.ReplaceAllString(str, replacement)
	s = filenameReservedWindowsNamesRegex.ReplaceAllString(s, replacement)
	return strings.ReplaceAll(s, " ", replacement)
}

// ParseVariable parses variable string.
func ParseVariable(value string) (string, error) {
	val, err := ParseCommand(os.ExpandEnv(value))
	if err != nil {
		return "", err
	}
	return val, nil
}

var tickerMatcher = regexp.MustCompile("`[^`]+`")

// ParseCommand substitutes command in the value string.
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

// MustTempDir returns temporary directory.
func MustTempDir(pattern string) string {
	t, err := os.MkdirTemp("", pattern)
	if err != nil {
		panic(err)
	}
	return t
}

// LogErr logs error if it's not nil.
func LogErr(action string, err error) {
	if err != nil {
		log.Printf("%s failed. %s", action, err)
	}
}

// TrunString returns truncated string.
func TruncString(val string, max int) string {
	if len(val) > max {
		return val[:max]
	}
	return val
}

// StringsWithFallback returns the first non-empty string
// in the parameter list.
func StringWithFallback(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

// MatchExtension returns true if extension matches.
func MatchExtension(file string, exts []string) bool {
	ext := filepath.Ext(file)
	for _, e := range exts {
		if e == ext {
			return true
		}
	}
	return false
}
