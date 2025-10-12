package core

import "regexp"

// DAGNameMaxLen defines the maximum allowed length for a DAG name.
const DAGNameMaxLen = 40

// dagNameRegex matches valid DAG names: alphanumeric, underscore, dash, dot.
var dagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// ValidateDAGName validates a DAG name according to shared rules.
// - Empty name is allowed (caller may provide one via context or filename).
// - Non-empty name must satisfy length and allowed character constraints.
func ValidateDAGName(name string) error {
	if name == "" {
		return nil
	}
	if len(name) > DAGNameMaxLen {
		return ErrNameTooLong
	}
	if !dagNameRegex.MatchString(name) {
		return ErrNameInvalidChars
	}
	return nil
}
