// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_StringParam(t *testing.T) {
	tests := []struct {
		name      string
		flagName  string
		flagValue string
		expected  string
		expectErr bool
	}{
		{
			name:      "StringParamWithoutQuotes",
			flagName:  "test-param",
			flagValue: "hello",
			expected:  "hello",
			expectErr: false,
		},
		{
			name:      "StringParamWithDoubleQuotes",
			flagName:  "test-param",
			flagValue: `"world"`,
			expected:  "world",
			expectErr: false,
		},
		{
			name:      "EmptyStringParam",
			flagName:  "test-param",
			flagValue: `""`,
			expected:  "",
			expectErr: false,
		},
		{
			name:      "StringParamWithEscapedDoubleQuotes",
			flagName:  "test-param",
			flagValue: `"{\"key\":\"value with \\\"quotes\\\"\"}"`, // This is the string literal `{"key":"value with \"quotes\""}`
			expected:  `{"key":"value with \"quotes\""}`,
			expectErr: false,
		},
		{
			name:      "JSONStringParam",
			flagName:  "test-param",
			flagValue: `"{ \"name\": \"test\", \"value\": 123 }"`, // This is the string literal `{ "name": "test", "value": 123 }`
			expected:  `{ "name": "test", "value": 123 }`,
			expectErr: false,
		},
		{
			name:      "FlagNotFound",
			flagName:  "non-existent-param",
			flagValue: "", // Value doesn't matter if flag not found
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use: "test",
			}
			if tt.flagName != "non-existent-param" { // Only add flag if it's expected to exist
				cmd.Flags().String(tt.flagName, "", "test flag")
				_ = cmd.Flags().Set(tt.flagName, tt.flagValue)
			}

			ctx := &Context{
				Command: cmd,
			}

			val, err := ctx.StringParam(tt.flagName)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error but got: %v", err)
				}
				if val != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, val)
				}
			}
		})
	}
}

func TestCheckUpdatesFlagBindingUsesSnakeCaseConfigKey(t *testing.T) {
	tests := []struct {
		name  string
		cmd   *cobra.Command
		flags []commandLineFlag
	}{
		{name: "server", cmd: Server(), flags: serverFlags},
		{name: "start-all", cmd: StartAll(), flags: startAllFlags},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.SetDefault("check_updates", true)
			bindFlags(v, tt.cmd, tt.flags...)

			require.NotNil(t, tt.cmd.Flags().Lookup("check-updates"))
			assert.True(t, v.GetBool("check_updates"))

			require.NoError(t, tt.cmd.Flags().Set("check-updates", "false"))
			assert.False(t, v.GetBool("check_updates"))
		})
	}
}
