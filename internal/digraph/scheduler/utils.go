package scheduler

import (
	"strconv"
	"strings"
)

// parseExitCodeFromError attempts to extract an exit code from an error string.
// It looks for patterns like "exit status N" and extracts the numeric value.
// Returns the exit code and a boolean indicating if an exit code was found.
func parseExitCodeFromError(errStr string) (int, bool) {
	if !strings.Contains(errStr, "exit status") {
		return 0, false
	}

	// Look for the last occurrence of "exit status" followed by a number
	parts := strings.Split(errStr, "exit status ")
	if len(parts) <= 1 {
		return 0, false
	}

	// Get the last part and extract the number
	lastPart := parts[len(parts)-1]

	// Extract the number from the beginning of the string
	numStr := ""
	for _, ch := range lastPart {
		if ch >= '0' && ch <= '9' {
			numStr += string(ch)
		} else {
			break
		}
	}

	if numStr == "" {
		return 0, false
	}

	code, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, false
	}

	return code, true
}
