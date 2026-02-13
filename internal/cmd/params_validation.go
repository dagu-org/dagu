package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/startvalidation"
)

func validateStartArgumentSeparator(ctx *Context, args []string) error {
	return startvalidation.ValidateArgumentSeparator(ctx.Command.ArgsLenAtDash() != -1, args)
}

func validateStartPositionalParamCount(ctx *Context, args []string, dag *core.DAG) error {
	input, err := buildStartValidationInput(ctx, args)
	if err != nil {
		return err
	}
	return startvalidation.ValidatePositionalParamCount(dag.DefaultParams, input)
}

func buildStartValidationInput(ctx *Context, args []string) (startvalidation.ParamInput, error) {
	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 {
		if argsLenAtDash >= len(args) {
			return startvalidation.ParamInput{}, nil
		}
		return startvalidation.ParamInput{DashArgs: args[argsLenAtDash:]}, nil
	}

	raw, err := ctx.Command.Flags().GetString("params")
	if err != nil {
		return startvalidation.ParamInput{}, fmt.Errorf("failed to get parameters: %w", err)
	}
	return startvalidation.ParamInput{RawParams: raw}, nil
}
