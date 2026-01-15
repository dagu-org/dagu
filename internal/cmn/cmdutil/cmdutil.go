package cmdutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
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
	// Handle empty args (e.g., "pwd " with trailing space)
	if args == "" {
		return command, nil
	}
	return command, strings.Split(args, ArgsDelimiter)
}

// GetShellCommand returns the shell to use for command execution
func GetShellCommand(configuredShell string) string {
	if configuredShell != "" {
		return configuredShell
	}

	// Check for global default shell via environment variable
	if defaultShell := os.Getenv("DAGU_DEFAULT_SHELL"); defaultShell != "" {
		return defaultShell
	}

	// Platform-specific default shell detection
	if runtime.GOOS == "windows" {
		return getWindowsDefaultShell()
	}

	// Unix-like systems: Try system shell first
	if systemShell := os.ExpandEnv("${SHELL}"); systemShell != "" {
		return systemShell
	}

	// Fallback to sh if available
	if shPath, err := exec.LookPath("sh"); err == nil {
		return shPath
	}

	return ""
}

// getWindowsDefaultShell returns the default shell for Windows systems
func getWindowsDefaultShell() string {
	// First check if SHELL environment variable is set (e.g., Git Bash, WSL)
	if systemShell := os.ExpandEnv("${SHELL}"); systemShell != "" {
		return systemShell
	}

	// Try PowerShell (preferred on Windows)
	if psPath, err := exec.LookPath("powershell"); err == nil {
		return psPath
	}

	// Try PowerShell Core (cross-platform PowerShell)
	if pwshPath, err := exec.LookPath("pwsh"); err == nil {
		return pwshPath
	}

	// Fallback to cmd.exe
	if cmdPath, err := exec.LookPath("cmd"); err == nil {
		return cmdPath
	}

	return ""
}

// SplitCommand splits a command string into a command and its arguments.
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

// unquoteToken removes surrounding quotes from a token if present
func unquoteToken(token string) string {
	if len(token) < 2 {
		return token
	}

	// Check for matching quotes at start and end
	if (token[0] == '"' && token[len(token)-1] == '"') ||
		(token[0] == '\'' && token[len(token)-1] == '\'') {
		// Try to unquote using strconv.Unquote for double quotes
		if token[0] == '"' {
			if unquoted, err := strconv.Unquote(token); err == nil {
				return unquoted
			}
		}
		// For single quotes, or if strconv.Unquote fails,
		// just remove the surrounding quotes
		return token[1 : len(token)-1]
	}

	// Don't unquote backticks - they're used for command substitution
	return token
}

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
	var inQuote, inSingleQuote, inBacktick, inEscape bool
	var current []rune
	var tokens []string

	runes := []rune(cmdString)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case inEscape:
			current = append(current, r)
			inEscape = false
		case r == '\\':
			current = append(current, r)
			inEscape = true
		case r == '"' && !inBacktick && !inSingleQuote:
			current = append(current, r)
			inQuote = !inQuote
		case r == '\'' && !inBacktick && !inQuote:
			current = append(current, r)
			inSingleQuote = !inSingleQuote
		case r == '`' && !inSingleQuote:
			current = append(current, r)
			inBacktick = !inBacktick
		case r == '|' && !inQuote && !inSingleQuote && !inBacktick:
			// Check if this is part of || operator
			if i+1 < len(runes) && runes[i+1] == '|' {
				// This is ||, not a pipe
				if len(current) > 0 {
					tokens = append(tokens, string(current))
					current = nil
				}
				tokens = append(tokens, "||")
				i++ // Skip the next |
			} else {
				// This is a single pipe
				if len(current) > 0 {
					tokens = append(tokens, string(current))
					current = nil
				}
				tokens = append(tokens, "|")
			}
		case r == '&' && !inQuote && !inSingleQuote && !inBacktick:
			// Check if this is part of && operator
			if i+1 < len(runes) && runes[i+1] == '&' {
				// This is &&
				if len(current) > 0 {
					tokens = append(tokens, string(current))
					current = nil
				}
				tokens = append(tokens, "&&")
				i++ // Skip the next &
			} else {
				// Single & (background operator)
				current = append(current, r)
			}
		case unicode.IsSpace(r) && !inQuote && !inSingleQuote && !inBacktick:
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
			// Unquote the token if it's quoted
			unquoted := unquoteToken(token)
			currentCmd = append(currentCmd, unquoted)
		}
	}

	if len(currentCmd) > 0 {
		pipeline = append(pipeline, currentCmd)
	}

	return pipeline, nil
}

// GetScriptExtension returns the appropriate file extension for the given shell.
// This is needed because some shells (like PowerShell) require specific file extensions.
func GetScriptExtension(shellCommand string) string {
	if shellCommand == "" {
		return ""
	}

	cmdLower := strings.ToLower(shellCommand)

	switch {
	case strings.HasSuffix(cmdLower, "powershell.exe"),
		strings.HasSuffix(cmdLower, "powershell"),
		strings.HasSuffix(cmdLower, "pwsh.exe"),
		strings.HasSuffix(cmdLower, "pwsh"):
		return ".ps1"

	case strings.HasSuffix(cmdLower, "cmd.exe"),
		strings.HasSuffix(cmdLower, "cmd"):
		return ".bat"

	case strings.HasSuffix(cmdLower, "bash.exe"),
		strings.HasSuffix(cmdLower, "bash"),
		strings.HasSuffix(cmdLower, "zsh"),
		strings.HasSuffix(cmdLower, "/sh"),
		strings.HasSuffix(cmdLower, "sh.exe"):
		return ".sh"

	// Exact match for "sh" only
	case cmdLower == "sh":
		return ".sh"

	default:
		return ""
	}
}

// DetectShebang checks if the given script starts with a shebang (#!) line.
func DetectShebang(script string) (string, []string, error) {
	reader := strings.NewReader(script)
	// Read the first two bytes to check for shebang
	buf := make([]byte, 2)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return "", nil, fmt.Errorf("failed to read script for shebang: %w", err)
	}
	// Check for shebang prefix "#!"
	if n < 2 || string(buf[:n]) != "#!" {
		return "", nil, nil
	}

	// Read the first line
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", nil, fmt.Errorf("failed to read shebang line: %w", err)
		}
		return "", nil, nil
	}
	line := scanner.Text()

	// Split the shebang line into command and args
	return SplitCommand(strings.TrimSpace(line))
}
