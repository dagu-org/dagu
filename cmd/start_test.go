package main

import (
	"fmt"
	"os"
	"testing"
)

func Test_startCommand(t *testing.T) {
	tests := []appTest{
		{
			args: []string{"", "start", testConfig("cmd_start_multiple_steps.yaml")}, errored: false,
			output: []string{"1 finished", "2 finished"},
		},
		{
			args: []string{"", "start", testConfig("cmd_start_with_params.yaml")}, errored: false,
			output: []string{"params is param-value"},
		},
		{
			args: []string{"", "start", "--params=x y", testConfig("cmd_start_with_params_2.yaml")}, errored: false,
			output: []string{"params are x and y"},
		},
		{
			args: []string{"", "start", testConfig("cmd_start_success")}, errored: false,
			output: []string{"1 finished"},
		},
		{
			args: []string{"", "start",
				fmt.Sprintf("--config=%s", testConfig("cmd_start_global_config.yaml")),
				testConfig("cmd_start_global_config_check.yaml")}, errored: false,
			output: []string{"GLOBAL_ENV_VAR"},
		},
	}

	// For testing --config parameter we need to set the environment variable for now.
	os.Setenv("TEST_CONFIG_BASE", testdataDir)

	for _, v := range tests {
		app := makeApp()
		runAppTestOutput(app, v, t)
	}
}
