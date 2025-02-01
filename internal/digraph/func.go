package digraph

import (
	"strconv"
)

// parseFuncCall parses the function call in the step definition.
// Deprecated: use subworkflow instead.
func parseFuncCall(step *Step, call *callFuncDef, funcs []*funcDef) error {
	if call == nil {
		return nil
	}

	passedArgs := make(map[string]string)
	step.Args = make([]string, 0, len(call.Args))

	for k, v := range call.Args {
		if strV, ok := v.(string); ok {
			step.Args = append(step.Args, strV)
			passedArgs[k] = strV
			continue
		}

		if intV, ok := v.(int); ok {
			strV := strconv.Itoa(intV)
			step.Args = append(step.Args, strV)
			passedArgs[k] = strV
			continue
		}

		return errArgsMustBeConvertibleToIntOrString
	}

	calledFuncDef := &funcDef{}

	for _, funcDef := range funcs {
		if funcDef.Name == call.Function {
			calledFuncDef = funcDef
			break
		}
	}

	step.Command = paramRegex.ReplaceAllString(calledFuncDef.Command, "")
	step.CmdWithArgs = assignValues(calledFuncDef.Command, passedArgs)

	return nil
}
