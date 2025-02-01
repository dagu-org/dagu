package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

// ArgsDelimiter is the delimiter used to separate command arguments
const ArgsDelimiter = "∯ᓰ♨"

// JoinCommandArgs joins a command and its arguments into a single string
// separated by ArgsDelimiter
func JoinCommandArgs(cmd string, args []string) string {
	return fmt.Sprintf("%s %s", cmd, strings.Join(args, ArgsDelimiter))
}

// SplitCommandArgs splits a command and its arguments into a command and a slice of arguments
func SplitCommandArgs(cmdWithArgs string) (string, []string) {
	parts := strings.SplitN(cmdWithArgs, " ", 2)
	if len(parts) == 1 {
		return parts[0], nil
	}
	command, args := parts[0], parts[1]
	return command, strings.Split(args, ArgsDelimiter)
}

// GetShellCommand returns the shell to use for command execution
func GetShellCommand(configuredShell string) string {
	if configuredShell != "" {
		return configuredShell
	}

	// Try system shell first
	if systemShell := os.ExpandEnv("${SHELL}"); systemShell != "" {
		return systemShell
	}

	// Fallback to sh if available
	if shPath, err := exec.LookPath("sh"); err == nil {
		return shPath
	}

	return ""
}

func SplitCommandWithSub(cmd string) (string, []string, error) {
	pipeline, err := ParsePipedCommand(cmd)
	if err != nil {
		return "", nil, err
	}

	for _, command := range pipeline {
		if len(command) < 2 {
			continue
		}
		for i, arg := range command {
			command[i] = arg
			// Escape the command
			command[i] = escapeReplacer.Replace(command[i])
			// Substitute command in the command.
			command[i], err = substituteCommands(command[i])
			if err != nil {
				return "", nil, fmt.Errorf("failed to substitute command: %w", err)
			}
		}
	}

	if len(pipeline) > 1 {
		first := pipeline[0]
		cmd := first[0]
		args := first[1:]
		for _, command := range pipeline[1:] {
			args = append(args, "|")
			args = append(args, command...)
		}
		return cmd, args, nil
	}

	if len(pipeline) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	command := pipeline[0]
	if len(command) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	return command[0], command[1:], nil
}

var (
	escapeReplacer = strings.NewReplacer(
		`\t`, `\\\\t`,
		`\r`, `\\\\r`,
		`\n`, `\\\\n`,
	)
)

func SplitCommand(cmd string) (string, []string, error) {
	pipeline, err := ParsePipedCommand(cmd)
	if err != nil {
		return "", nil, err
	}

	if len(pipeline) > 1 {
		first := pipeline[0]
		cmd := first[0]
		args := first[1:]
		for _, command := range pipeline[1:] {
			args = append(args, "|")
			args = append(args, command...)
		}
		return cmd, args, nil
	}

	if len(pipeline) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	command := pipeline[0]
	if len(command) == 0 {
		return "", nil, ErrCommandIsEmpty
	}

	return command[0], command[1:], nil
}

var ErrCommandIsEmpty = fmt.Errorf("command is empty")

// ParsePipedCommand splits a shell-style command string into a pipeline ([][]string).
// Each sub-slice represents a single command. Unquoted "|" tokens define the boundaries.
//
// Example:
//
//	parsePipedCommand(`echo foo | grep foo | wc -l`) =>
//	  [][]string{
//	    {"echo", "foo"},
//	    {"grep", "foo"},
//	    {"wc", "-l"},
//	  }
//
//	parsePipedCommand(`echo "hello|world"`) =>
//	  [][]string{ {"echo", "hello|world"} } // single command
func ParsePipedCommand(cmdString string) ([][]string, error) {
	var inQuote, inBacktick, inEscape bool
	var current []rune
	var tokens []string

	for _, r := range cmdString {
		switch {
		case inEscape:
			current = append(current, r)
			inEscape = false
		case r == '\\':
			current = append(current, r)
			inEscape = true
		case r == '"' && !inBacktick:
			current = append(current, r)
			inQuote = !inQuote
		case r == '`':
			current = append(current, r)
			inBacktick = !inBacktick
		case r == '|' && !inQuote && !inBacktick:
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
			tokens = append(tokens, "|")
		case unicode.IsSpace(r) && !inQuote && !inBacktick:
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		default:
			current = append(current, r)
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	var pipeline [][]string
	var currentCmd []string

	for _, token := range tokens {
		if token == "|" {
			if len(currentCmd) > 0 {
				pipeline = append(pipeline, currentCmd)
				currentCmd = nil
			}
		} else {
			currentCmd = append(currentCmd, token)
		}
	}

	if len(currentCmd) > 0 {
		pipeline = append(pipeline, currentCmd)
	}

	return pipeline, nil
}
