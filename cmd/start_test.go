package cmd

import (
	"testing"
)

func Test_startCommand(t *testing.T) {
	tests := []appTest{
		{
			args: []string{testConfig("cmd_start_multiple_steps.yaml")}, errored: false,
			output: []string{"1 finished", "2 finished"},
		},
		{
			args: []string{testConfig("cmd_start_with_params.yaml")}, errored: false,
			output: []string{"params is param-value"},
		},
		{
			args: []string{testConfig("cmd_start_with_params_2.yaml")}, errored: false,
			flags:  map[string]string{"params": "x y"},
			output: []string{"params are x and y"},
		},
	}

	for _, v := range tests {
		cmd := startCmd
		runCmdTestOutput(cmd, v, t)
	}
}
