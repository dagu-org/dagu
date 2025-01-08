package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// tickerMatcher matches the command in the value string.
// Example: "`date`"
var tickerMatcher = regexp.MustCompile("`[^`]+`")

// substituteCommands substitutes command in the value string.
// This logic needs to be refactored to handle more complex cases.
func substituteCommands(input string) (string, error) {
	matches := tickerMatcher.FindAllString(strings.TrimSpace(input), -1)
	if matches == nil {
		return input, nil
	}

	ret := input
	for i := 0; i < len(matches); i++ {
		// Execute the command and replace the command with the output.
		command := matches[i]

		sh := GetShellCommand("")
		cmd := exec.Command(sh, "-c", command[1:len(command)-1])
		cmd.Env = os.Environ()
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to execute command: %w", err)
		}

		ret = strings.ReplaceAll(ret, command, strings.TrimSpace(string(out)))
	}

	return ret, nil
}
