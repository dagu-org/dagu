// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package util

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"
)

var (
	ErrUnexpectedEOF = errors.New(
		"unexpected end of input after escape character",
	)
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

const (
	legacyTimeFormat = "2006-01-02 15:04:05"
	timeEmpty        = "-"
)

// FormatTime returns formatted time.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return timeEmpty
	}

	return t.Format(time.RFC3339)
}

// ParseTime parses time string.
func ParseTime(val string) (time.Time, error) {
	if val == timeEmpty {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation(time.RFC3339, val, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation(legacyTimeFormat, val, time.Local)
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

const splitCmdN = 2

// SplitCommandWithParse splits command string to program and arguments.
func SplitCommandWithParse(cmd string) (cmdx string, args []string) {
	splits := strings.SplitN(cmd, " ", splitCmdN)
	if len(splits) == 1 {
		return splits[0], []string{}
	}

	cmdx = splits[0]

	parser := shellwords.NewParser()
	parser.ParseBacktick = true
	parser.ParseEnv = false

	args, err := parser.Parse(escapeReplacer.Replace(splits[1]))
	if err != nil {
		log.Printf("failed to parse arguments: %s", err)
		// if parse shell world error use all string as argument
		return cmdx, []string{splits[1]}
	}

	var ret []string
	for _, v := range args {
		ret = append(ret, os.ExpandEnv(unescapeReplacer.Replace(v)))
	}

	return cmdx, ret
}

// SplitCommand splits command string to program and arguments.
func SplitCommand(cmd string) (cmdx string, args []string) {
	splits := strings.SplitN(cmd, " ", splitCmdN)
	if len(splits) == 1 {
		return splits[0], []string{}
	}

	return splits[0], strings.Fields(splits[1])
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
	filenameReservedRegex = regexp.MustCompile(
		`[<>:"/\\|?*\x00-\x1F]`,
	)
	filenameReservedWindowsNamesRegex = regexp.MustCompile(
		`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`,
	)
	filenameSpacingRegex = regexp.MustCompile(`\s`)
	specialCharRepl      = "_"
)

// ValidFilename makes filename valid by replacing reserved characters.
func ValidFilename(str string) string {
	s := filenameReservedRegex.ReplaceAllString(str, specialCharRepl)
	s = filenameReservedWindowsNamesRegex.ReplaceAllString(s, specialCharRepl)
	return filenameSpacingRegex.ReplaceAllString(s, specialCharRepl)
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

// AddYamlExtension adds .yaml extension if not present
// if it has .yml extension, replace it with .yaml
func AddYamlExtension(file string) string {
	ext := filepath.Ext(file)
	if ext == "" {
		return file + ".yaml"
	}
	if ext == ".yml" {
		return strings.TrimSuffix(file, ext) + ".yaml"
	}
	return file
}
