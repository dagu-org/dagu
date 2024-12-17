// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"errors"
	"log"
	"os"
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
