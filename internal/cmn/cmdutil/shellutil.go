package cmdutil

import "strings"

// HasShellArgs reports whether a shell slice contains a non-empty argument
// that is not the "direct" shell placeholder.
func HasShellArgs(shell []string) bool {
	for _, arg := range shell {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if isDirectShell(arg) {
			return false
		}
		return true
	}
	return false
}

// IsShellValueSet checks whether a shell value from a generic config is non-empty
// and not the "direct" shell placeholder.
func IsShellValueSet(shellValue any) bool {
	switch v := shellValue.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		return trimmed != "" && !isDirectShell(trimmed)
	case []string:
		return HasShellArgs(v)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				if isDirectShell(s) {
					return false
				}
				return true
			} else if item != nil {
				return true
			}
		}
	}
	return false
}

func isDirectShell(value string) bool {
	return value == "direct"
}
