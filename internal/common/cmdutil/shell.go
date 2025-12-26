package cmdutil

import (
	"strings"
)

// ShellQuote escapes a string for use in a shell command.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}

	// Use a conservative set of safe characters:
	// Alphanumeric, hyphen, underscore, dot, and slash.
	// We only consider ASCII alphanumeric as safe to avoid locale-dependent behavior.
	safe := true
	for i := 0; i < len(s); i++ {
		b := s[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
			b == '-' || b == '_' || b == '.' || b == '/' {
			continue
		}
		safe = false
		break
	}
	if safe {
		return s
	}

	// Wrap in single quotes and escape any internal single quotes.
	// This is the most robust way to escape for POSIX-compliant shells.
	// 'user's file' -> 'user'\''s file'
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ShellQuoteArgs escapes a slice of strings for use in a shell command.
func ShellQuoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = ShellQuote(arg)
	}
	return strings.Join(quoted, " ")
}
