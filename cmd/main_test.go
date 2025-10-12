package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/cli"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name           string
		args           []string
		expectError    bool
		expectContains []string
		setup          func()
		cleanup        func()
	}{
		{
			name:        "HelpCommand",
			args:        []string{"dagu", "--help"},
			expectError: false,
			expectContains: []string{
				"Dagu is a compact, portable workflow engine",
				"declarative model for orchestrating command execution",
			},
		},
		{
			name:        "InvalidCommand",
			args:        []string{"dagu", "invalid-command"},
			expectError: true,
			expectContains: []string{
				"unknown command",
			},
		},
		{
			name:        "NoArguments",
			args:        []string{"dagu"},
			expectError: false,
			expectContains: []string{
				"Dagu is a compact, portable workflow engine",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			// Capture output
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)

			// Reset command for fresh state
			rootCmd.ResetFlags()
			rootCmd.ResetCommands()

			// Re-add commands
			rootCmd.AddCommand(cli.Start())
			rootCmd.AddCommand(cli.Enqueue())
			rootCmd.AddCommand(cli.Dequeue())
			rootCmd.AddCommand(cli.Stop())
			rootCmd.AddCommand(cli.Restart())
			rootCmd.AddCommand(cli.Dry())
			rootCmd.AddCommand(cli.Validate())
			rootCmd.AddCommand(cli.Status())
			rootCmd.AddCommand(cli.Version())
			rootCmd.AddCommand(cli.Server())
			rootCmd.AddCommand(cli.Scheduler())
			rootCmd.AddCommand(cli.Retry())
			rootCmd.AddCommand(cli.StartAll())
			rootCmd.AddCommand(cli.Migrate())

			// Set args
			rootCmd.SetArgs(tt.args[1:]) // Skip program name

			// Execute
			err := rootCmd.Execute()

			// Check error
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check output
			output := buf.String()
			for _, expected := range tt.expectContains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestMainFunction(t *testing.T) {
	// Test that the main function properly handles commands
	// Since main() calls os.Exit, we can't test it directly
	// Instead we test the command execution through rootCmd

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "SuccessfulCommand",
			args:        []string{"version"},
			expectError: false,
		},
		{
			name:        "FailedCommand",
			args:        []string{"invalid-command-that-does-not-exist"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset root command
			resetRootCommand()

			// Set args
			rootCmd.SetArgs(tt.args)

			// Execute
			err := rootCmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInit(t *testing.T) {
	// Test that init() sets up the version correctly
	// Note: init() is automatically called by Go
	// Since the test version is set to "0.0.0", that's what should be in config.Version
	assert.NotEmpty(t, config.Version)
}

func TestRootCommandStructure(t *testing.T) {
	// Test that all expected commands are registered
	expectedCommands := []string{
		"start",
		"enqueue",
		"dequeue",
		"stop",
		"restart",
		"dry",
		"validate",
		"status",
		"version",
		"server",
		"scheduler",
		"retry",
		"start-all",
		"migrate",
	}

	// Get all commands
	commands := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		commands[cmd.Name()] = true
	}

	// Check all expected commands exist
	for _, expected := range expectedCommands {
		assert.True(t, commands[expected], "Command %s not found", expected)
	}
}

func TestRootCommandMetadata(t *testing.T) {
	assert.Equal(t, config.AppSlug, rootCmd.Use)
	assert.Equal(t, "Dagu is a compact, portable workflow engine", rootCmd.Short)
	assert.Contains(t, rootCmd.Long, "declarative model for orchestrating command execution")
	assert.Contains(t, rootCmd.Long, "shell scripts, Python commands, containerized")
}

func TestCommandHelp(t *testing.T) {
	// Test that each command has help text
	for _, cmd := range rootCmd.Commands() {
		t.Run(cmd.Name(), func(t *testing.T) {
			// Each command should have a short description
			assert.NotEmpty(t, cmd.Short, "Command %s missing short description", cmd.Name())

			// Test help output
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			// Reset args before setting help flag
			cmd.ResetFlags()

			// Execute help
			cmd.HelpFunc()(cmd, []string{})

			output := buf.String()
			assert.Contains(t, output, cmd.Name())
			assert.Contains(t, output, "Usage:")
		})
	}
}

// Helper to reset root command state
func resetRootCommand() {
	rootCmd = &cobra.Command{
		Use:   config.AppSlug,
		Short: "Dagu is a compact, portable workflow engine",
		Long: `Dagu is a compact, portable workflow engine.

It provides a declarative model for orchestrating command execution across
diverse environments, including shell scripts, Python commands, containerized
operations, or remote commands.
`,
	}

	// Re-add all commands
	rootCmd.AddCommand(cli.Start())
	rootCmd.AddCommand(cli.Enqueue())
	rootCmd.AddCommand(cli.Dequeue())
	rootCmd.AddCommand(cli.Stop())
	rootCmd.AddCommand(cli.Restart())
	rootCmd.AddCommand(cli.Dry())
	rootCmd.AddCommand(cli.Validate())
	rootCmd.AddCommand(cli.Status())
	rootCmd.AddCommand(cli.Version())
	rootCmd.AddCommand(cli.Server())
	rootCmd.AddCommand(cli.Scheduler())
	rootCmd.AddCommand(cli.Retry())
	rootCmd.AddCommand(cli.StartAll())
	rootCmd.AddCommand(cli.Migrate())
}
