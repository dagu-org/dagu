package main

import (
	"testing"
)

func Test_versionCommand(t *testing.T) {
	tests := []appTest{
		{
			args: []string{"", "version"}, errored: false,
			exactOutput: "0.0.1\n",
		},
	}

	for _, v := range tests {
		app := makeApp()
		runAppTestOutput(app, v, t)
	}
}
