package util

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/mattn/go-shellwords"
)

var (
	ErrUnexpectedEOF         = errors.New("unexpected end of input after escape character")
	ErrUnknownEscapeSequence = errors.New("unknown escape sequence")
)

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
	}

	return t.Format(constants.TimeFormat)
}

// ParseTime parses time string.
func ParseTime(val string) (time.Time, error) {
	if val == constants.TimeEmpty {
		return time.Time{}, nil
	}
	return time.ParseInLocation(constants.TimeFormat, val, time.Local)
}

var (
	escapeReplacer = strings.NewReplacer(
		`\t`, `\\t`,
		`\r`, `\\r`,
		`\n`, `\\n`,
	)
	unescapeReplacer = strings.NewReplacer(
		`\\t`, `\t`,
		`\\r`, `\r`,
		`\\n`, `\n`,
	)
)

// SplitCommand splits command string to program and arguments.
// TODO: This function needs to be refactored to handle more complex cases.
func SplitCommand(cmd string, parse bool) (cmdx string, args []string) {
	splits := strings.SplitN(cmd, " ", 2)
	if len(splits) == 1 {
		return splits[0], []string{}
	}

	cmdx = splits[0]

	parser := shellwords.NewParser()
	parser.ParseBacktick = parse
	parser.ParseEnv = false

	args, err := parser.Parse(escapeReplacer.Replace(splits[1]))
	if err != nil {
		log.Printf("failed to parse arguments: %s", err)
		// if parse shell world error use all string as argument
		return cmdx, []string{splits[1]}
	}

	var ret []string
	for _, v := range args {
		val := unescapeReplacer.Replace(v)
		if parse {
			val = os.ExpandEnv(val)
		}
		ret = append(ret, val)
	}

	return cmdx, ret
}

// FileExists returns true if file exists.
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

// OpenOrCreateFile opens file or creates it if it doesn't exist.
func OpenOrCreateFile(file string) (*os.File, error) {
	if FileExists(file) {
		return openFile(file)
	}
	return createFile(file)
}

// openFile opens file.
func openFile(file string) (*os.File, error) {
	outfile, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}
	return outfile, nil
}

// createFile creates file.
func createFile(file string) (*os.File, error) {
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

// ValidFilename makes filename valid by replacing reserved characters.
func ValidFilename(str, replacement string) string {
	s := filenameReservedRegex.ReplaceAllString(str, replacement)
	s = filenameReservedWindowsNamesRegex.ReplaceAllString(s, replacement)
	return strings.ReplaceAll(s, " ", replacement)
}

// MustTempDir returns temporary directory.
// This function is used only for testing.
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

// TruncString TurnString returns truncated string.
func TruncString(val string, max int) string {
	if len(val) > max {
		return val[:max]
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
