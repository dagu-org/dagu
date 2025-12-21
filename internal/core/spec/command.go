package spec

import (
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
)

// buildCommand parses the command field in the step definition.
// Case 1: command is nil
// Case 2: command is a string
// Case 3: command is an array
//
// In case 3, the first element is the command and the rest are the arguments.
// If the arguments are not strings, they are converted to strings.
//
// Example:
// ```yaml
// step:
//   - name: "echo hello"
//     command: "echo hello"
//
// ```
// or
// ```yaml
// step:
//   - name: "echo hello"
//     command: ["echo", "hello"]
//
// ```
// It returns an error if the command is not nil but empty.
func buildCommand(_ StepBuildContext, def step, st *core.Step) error {
	command := def.Command

	// Case 1: command is nil
	if command == nil {
		return nil
	}

	switch val := command.(type) {
	case string:
		// Case 2: command is a string
		val = strings.TrimSpace(val)
		if val == "" {
			return core.NewValidationError("command", val, ErrStepCommandIsEmpty)
		}

		// If the value is multi-line, treat it as a script
		if strings.Contains(val, "\n") {
			st.Script = val
			return nil
		}

		// We need to split the command into command and args.
		st.CmdWithArgs = val
		cmd, args, err := cmdutil.SplitCommand(val)
		if err != nil {
			return core.NewValidationError("command", val, fmt.Errorf("failed to parse command: %w", err))
		}
		st.Command = strings.TrimSpace(cmd)
		st.Args = args

	case []any:
		// Case 3: command is an array

		var command string
		var args []string
		for _, v := range val {
			val, ok := v.(string)
			if !ok {
				// If the value is not a string, convert it to a string.
				// This is useful when the value is an integer for example.
				val = fmt.Sprintf("%v", v)
			}
			val = strings.TrimSpace(val)
			if command == "" {
				command = val
				continue
			}
			args = append(args, val)
		}

		// Setup CmdWithArgs (this will be actually used in the command execution)
		var sb strings.Builder
		for i, arg := range st.Args {
			if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(fmt.Sprintf("%q", arg))
		}

		st.Command = command
		st.Args = args
		st.CmdWithArgs = fmt.Sprintf("%s %s", st.Command, sb.String())
		st.CmdArgsSys = cmdutil.JoinCommandArgs(st.Command, st.Args)

	default:
		// Unknown type for command field.
		return core.NewValidationError("command", val, ErrStepCommandMustBeArrayOrString)

	}

	return nil
}
