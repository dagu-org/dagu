package dag

import "strings"

// assertFunctions validates the function definitions.
func assertFunctions(fns []*funcDef) error {
	if fns == nil {
		return nil
	}

	nameMap := make(map[string]bool)
	for _, funcDef := range fns {
		if _, exists := nameMap[funcDef.Name]; exists {
			return errDuplicateFunction
		}
		nameMap[funcDef.Name] = true

		definedParamNames := strings.Split(funcDef.Params, " ")
		passedParamNames := extractParamNames(funcDef.Command)
		if len(definedParamNames) != len(passedParamNames) {
			return errFuncParamsMismatch
		}

		for i := 0; i < len(definedParamNames); i++ {
			if definedParamNames[i] != passedParamNames[i] {
				return errFuncParamsMismatch
			}
		}
	}

	return nil
}

// assertStepDef validates the step definition.
func assertStepDef(def *stepDef, funcs []*funcDef) error {
	// Step name is required.
	if def.Name == "" {
		return errStepNameRequired
	}

	// TODO: Validate executor config for each executor type.
	if def.Executor == nil && def.Command == nil && def.Call == nil &&
		def.Run == "" {
		return errStepCommandOrCallRequired
	}

	// validate the function call if it exists.
	if def.Call != nil {
		calledFunc := def.Call.Function
		calledFuncDef := &funcDef{}
		for _, funcDef := range funcs {
			if funcDef.Name == calledFunc {
				calledFuncDef = funcDef
				break
			}
		}
		if calledFuncDef.Name == "" {
			return errCallFunctionNotFound
		}

		definedParamNames := strings.Split(calledFuncDef.Params, " ")
		if len(def.Call.Args) != len(definedParamNames) {
			return errNumberOfParamsMismatch
		}

		for _, paramName := range definedParamNames {
			_, exists := def.Call.Args[paramName]
			if !exists {
				return errRequiredParameterNotFound
			}
		}
	}

	return nil
}
