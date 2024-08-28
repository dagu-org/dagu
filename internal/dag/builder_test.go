// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dag

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilder_Build(t *testing.T) {
	tests := []testCase{
		{
			Name:        "NoName",
			InputFile:   "no_name.yaml",
			ExpectedErr: errStepNameRequired,
		},
		{
			Name:        "NoCommand",
			InputFile:   "no_command.yaml",
			ExpectedErr: errStepCommandOrCallRequired,
		},
		{
			Name:        "InvalidEnv",
			InputFile:   "invalid_env.yaml",
			ExpectedErr: errInvalidEnvValue,
		},
		{
			Name:        "InvalidParams",
			InputFile:   "invalid_params.yaml",
			ExpectedErr: errInvalidParamValue,
		},
		{
			Name:        "InvalidSchedule",
			InputFile:   "invalid_schedule.yaml",
			ExpectedErr: errInvalidSchedule,
		},
		{
			Name:      "ValidEnv",
			InputFile: "valid_env.yaml",
			Expected: map[string]any{
				"env": map[string]string{"FOO": "123"},
			},
		},
		{
			Name:      "ValidEnvWithSubstitution",
			InputFile: "valid_env_substitution.yaml",
			Expected: map[string]any{
				"env": map[string]string{"VAR": "123"},
			},
		},
		{
			Name:      "ValidEnvWithSubstitutionAndEnv",
			InputFile: "valid_env_substitution_and_env.yaml",
			Expected: map[string]any{
				"env": map[string]string{"FOO": "BAR:BAZ:BAR:FOO"},
			},
		},
		{
			Name:      "ValidCommand",
			InputFile: "valid_command.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"command": "echo",
						"args":    []string{"1"},
						"name":    "step 1",
					},
				},
			},
		},
		{
			Name:      "ValidCommandInArray",
			InputFile: "valid_command_in_array.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"command": "echo",
						"args":    []string{"1"},
						"name":    "step 1",
					},
				},
			},
		},
		{
			Name:      "ValidCommandInList",
			InputFile: "valid_command_in_list.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"command": "echo",
						"args":    []string{"1"},
						"name":    "step 1",
					},
				},
			},
		},
		{
			Name:      "ValidTags",
			InputFile: "valid_tags.yaml",
			Expected: map[string]any{
				"tags": []string{"daily", "monthly"},
			},
		},
		{
			Name:      "ValidTagsList",
			InputFile: "valid_tags_list.yaml",
			Expected: map[string]any{
				"tags": []string{"daily", "monthly"},
			},
		},
		{
			Name:      "ValidSchedule",
			InputFile: "valid_schedule.yaml",
			Expected: map[string]any{
				"schedule": map[string][]string{
					"start":   {"0 1 * * *"},
					"stop":    {"0 2 * * *"},
					"restart": {"0 12 * * *"},
				},
			},
		},
		{
			Name:      "ScheduleInList",
			InputFile: "schedule_in_list.yaml",
			Expected: map[string]any{
				"schedule": map[string][]string{
					"start": {
						"0 1 * * *",
						"0 18 * * *",
					},
				},
			},
		},
		{
			Name:      "ScheduleWithMultipleValues",
			InputFile: "schedule_with_multiple_values.yaml",
			Expected: map[string]any{
				"schedule": map[string][]string{
					"start": {
						"0 1 * * *",
						"0 18 * * *",
					},
					"stop": {
						"0 2 * * *",
						"0 20 * * *",
					},
					"restart": {
						"0 12 * * *",
						"0 22 * * *",
					},
				},
			},
		},
		{
			Name:      "HTTPExecutor",
			InputFile: "http_executor.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"executor": "http",
					},
				},
			},
		},
		{
			Name:      "HTTPExecutorWithConfig",
			InputFile: "http_executor_with_config.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"executor": "http",
						"executorConfig": map[string]any{
							"key": "value",
							"map": map[string]any{
								"foo": "bar",
							},
						},
					},
				},
			},
		},
		{
			Name:      "SignalOnStop",
			InputFile: "signal_on_stop.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"signalOnStop": "SIGINT",
					},
				},
			},
		},
		{
			Name:      "ParamsWithSubstitution",
			InputFile: "params_with_substitution.yaml",
			Expected: map[string]any{
				"env": map[string]string{
					"1": "x",
					"2": "x",
				},
			},
		},
		{
			Name:      "ParamsWithQuotedValues",
			InputFile: "params_with_quoted_values.yaml",
			Expected: map[string]any{
				"env": map[string]string{
					"x": "a b c",
					"y": "d e f",
				},
			},
		},
		{
			Name:      "ParamsWithComplexValues",
			InputFile: "params_with_complex_values.yaml",
			Expected: map[string]any{
				"env": map[string]string{
					"P1": "foo",
					"P2": "BAR",
					"P3": "BAR",
					"X":  "bar",
					"Y":  "foo",
					"Z":  "A B C",
					"1":  "first",
					"2":  `P1=foo`,
					"3":  `P2=BAR`,
					"4":  `P3=BAR`,
					"5":  `X=bar`,
					"6":  `Y=foo`,
					"7":  `Z=A B C`,
				},
			},
		},
		{
			Name:      "ValidHandlers",
			InputFile: "valid_handlers.yaml",
			Expected: map[string]any{
				"handlers": map[string]stepTestCase{
					"exit": {
						"name":    "onExit",
						"command": "echo",
						"args":    []string{"exit"},
					},
					"success": {
						"name":    "onSuccess",
						"command": "echo",
						"args":    []string{"success"},
					},
					"failure": {
						"name":    "onFailure",
						"command": "echo",
						"args":    []string{"failure"},
					},
					"cancel": {
						"name":    "onCancel",
						"command": "echo",
						"args":    []string{"cancel"},
					},
				},
			},
		},
		{
			Name:      "ValidMailConfig",
			InputFile: "valid_mail_config.yaml",
			Expected: map[string]any{
				"smtp": map[string]string{
					"host":     "smtp.example.com",
					"port":     "587",
					"username": "user@example.com",
					"password": "password",
				},
				"errorMail": map[string]any{
					"from":       "error@example.com",
					"to":         "admin@example.com",
					"prefix":     "[ERROR]",
					"attachLogs": true,
				},
				"infoMail": map[string]any{
					"from":       "info@example.com",
					"to":         "user@example.com",
					"prefix":     "[INFO]",
					"attachLogs": false,
				},
			},
		},
		{
			Name:      "ValidSubWorkflow",
			InputFile: "valid_subworkflow.yaml",
			Expected: map[string]any{
				"steps": []stepTestCase{
					{
						"name":     "sub_workflow_step",
						"command":  "run",
						"args":     []string{"sub_dag", "param1=value1 param2=value2"},
						"executor": "subworkflow",
						"subWorkflow": map[string]string{
							"name":   "sub_dag",
							"params": "param1=value1 param2=value2",
						},
					},
				},
			},
		},
		{
			Name:      "ValidMiscs",
			InputFile: "valid_miscs.yaml",
			Expected: map[string]any{
				"histRetentionDays": 7,
				"maxActiveRuns":     3,
				"maxCleanUpTime":    time.Duration(300 * time.Second),
				"preconditions": []Condition{
					{Condition: "test -f file.txt", Expected: "true"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTest(t, tc)
		})
	}
}

func TestOverrideBaseConfig(t *testing.T) {
	// Base config has the following values:
	// MailOn: {Failure: true, Success: false}
	t.Run("Override", func(t *testing.T) {
		baseConfig := filepath.Join(testdataDir, "base.yaml")

		// Overwrite the base config with the following values:
		// MailOn: {Failure: false, Success: false}
		dg, err := Load(baseConfig, filepath.Join(testdataDir, "overwrite.yaml"), "")
		require.NoError(t, err)

		// The MailOn key should be overwritten.
		require.Equal(t, &MailOn{Failure: false, Success: false}, dg.MailOn)
		require.Equal(t, dg.HistRetentionDays, 7)
	})
	t.Run("WithoutOverride", func(t *testing.T) {
		baseConfig := filepath.Join(testdataDir, "base.yaml")

		// no_overwrite.yaml does not have the MailOn key.
		dg, err := Load(baseConfig, filepath.Join(testdataDir, "no_overwrite.yaml"), "")
		require.NoError(t, err)

		// The MailOn key should be the same as the base config.
		require.Equal(t, &MailOn{Failure: true, Success: false}, dg.MailOn)
		require.Equal(t, dg.HistRetentionDays, 30)
	})
}

type testCase struct {
	Name        string
	InputFile   string
	Expected    map[string]any
	ExpectedErr error
}

type stepTestCase map[string]any

func readTestFile(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir, filename))
	require.NoError(t, err)
	return data
}

func runTest(t *testing.T, tc testCase) {
	t.Helper()
	dag, err := loadYAML(readTestFile(t, tc.InputFile), buildOpts{})

	if tc.ExpectedErr != nil {
		assert.Error(t, err)
		if errs, ok := err.(*errorList); ok && len(*errs) > 0 {
			// check if the error is in the list of errors
			found := false
			for _, e := range *errs {
				if errors.Is(e, tc.ExpectedErr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error %v, got %v", tc.ExpectedErr, err)
			}
		} else if !errors.Is(err, tc.ExpectedErr) {
			t.Errorf("expected error %v, got %v", tc.ExpectedErr, err)
		}
		return
	}

	require.NoError(t, err)
	for k, v := range tc.Expected {
		switch k {
		case "steps":
			stepTestCases := v.([]stepTestCase)
			require.Len(t, dag.Steps, len(stepTestCases))
			for i, step := range dag.Steps {
				testStep(t, step, stepTestCases[i])
			}
		case "env":
			for envKey, envVal := range v.(map[string]string) {
				assert.Equal(t, envVal, os.Getenv(envKey))
			}
		case "tags":
			for _, tag := range v.([]string) {
				assert.True(t, dag.HasTag(tag))
			}
		case "schedule":
			schedules := v.(map[string][]string)
			for scheduleType, expressions := range schedules {
				var actual []Schedule
				switch scheduleKey(scheduleType) {
				case scheduleKeyStart:
					actual = dag.Schedule
				case scheduleKeyStop:
					actual = dag.StopSchedule
				case scheduleKeyRestart:
					actual = dag.RestartSchedule
				}
				assert.Len(t, actual, len(expressions))
				for i, expr := range expressions {
					assert.Equal(t, expr, actual[i].Expression)
				}
			}
		case "histRetentionDays":
			assert.Equal(t, v.(int), dag.HistRetentionDays)
		case "maxActiveRuns":
			assert.Equal(t, v.(int), dag.MaxActiveRuns)
		case "maxCleanUpTime":
			assert.Equal(t, v.(time.Duration), dag.MaxCleanUpTime)
		case "preconditions":
			assert.Equal(t, v.([]Condition), dag.Preconditions)
		case "handlers":
			for handlerName, handler := range v.(map[string]stepTestCase) {
				switch handlerName {
				case "exit":
					testStep(t, *dag.HandlerOn.Exit, handler)
				case "success":
					testStep(t, *dag.HandlerOn.Success, handler)
				case "failure":
					testStep(t, *dag.HandlerOn.Failure, handler)
				case "cancel":
					testStep(t, *dag.HandlerOn.Cancel, handler)
				default:
					panic("unexpected handler: " + handlerName)
				}
			}
		case "smtp":
			for key, val := range v.(map[string]string) {
				switch key {
				case "host":
					assert.Equal(t, val, dag.SMTP.Host)
				case "port":
					assert.Equal(t, val, dag.SMTP.Port)
				case "username":
					assert.Equal(t, val, dag.SMTP.Username)
				case "password":
					assert.Equal(t, val, dag.SMTP.Password)
				default:
					panic("unexpected smtp key: " + key)
				}
			}
		case "errorMail":
			testMailConfig(t, *dag.ErrorMail, v.(map[string]any))
		case "infoMail":
			testMailConfig(t, *dag.InfoMail, v.(map[string]any))
		default:
			panic("unexpected key: " + k)
		}
	}
}

func testMailConfig(t *testing.T, mailConfig MailConfig, tc map[string]any) {
	for key, val := range tc {
		switch key {
		case "from":
			assert.Equal(t, val, mailConfig.From)
		case "to":
			assert.Equal(t, val, mailConfig.To)
		case "prefix":
			assert.Equal(t, val, mailConfig.Prefix)
		case "attachLogs":
			assert.Equal(t, val, mailConfig.AttachLogs)
		default:
			t.Errorf("unexpected mail key: %s", key)
		}
	}
}

func testStep(t *testing.T, step Step, tc stepTestCase) {
	for k, v := range tc {
		switch k {
		case "name":
			assert.Equal(t, v.(string), step.Name)
		case "command":
			assert.Equal(t, v.(string), step.Command)
		case "args":
			assert.Equal(t, v.([]string), step.Args)
		case "executorConfig":
			assert.Equal(t, v.(map[string]any), step.ExecutorConfig.Config)
		case "executor":
			assert.Equal(t, v.(string), step.ExecutorConfig.Type)
		case "signalOnStop":
			assert.Equal(t, v.(string), step.SignalOnStop)
		case "subWorkflow":
			for k, val := range v.(map[string]string) {
				switch k {
				case "name":
					assert.Equal(t, val, step.SubWorkflow.Name)
				case "params":
					assert.Equal(t, val, step.SubWorkflow.Params)
				default:
					panic("unexpected subworkflow key: " + k)
				}
			}
		default:
			panic("unexpected key: " + k)
		}
	}
}
