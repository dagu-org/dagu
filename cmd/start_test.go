package main

import (
	"testing"
)

func Test_startCommand(t *testing.T) {
	tests := []appTest{
		{
			args: []string{"", "start", testConfig("multiple_steps.yaml")}, errored: false,
			output: []string{"1 finished", "2 finished"},
		},
		{
			args: []string{"", "start", testConfig("basic_failure.yaml")}, errored: true,
			output: []string{"1 failed"},
		},
		{
			args: []string{"", "start", testConfig("with_params.yaml")}, errored: false,
			output: []string{"params is param-value"},
		},
		{
			args: []string{"", "start", "--params=x y", testConfig("with_params_2.yaml")}, errored: false,
			output: []string{"params are x and y"},
		},
	}

	for _, v := range tests {
		app := makeApp()
		runAppTestOutput(app, v, t)
	}
}
