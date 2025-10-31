package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/spf13/cobra"
)

// Validate creates the 'validate' CLI command that checks a DAG spec for errors.
//
// It follows the same validation logic used by the API's UpdateDAGSpec handler:
// - Load the YAML without evaluation
// - Run DAG.Validate()
//
// The command prints validation results and any errors found.
// Unlike other commands, this does NOT use NewCommand wrapper to allow proper
// error handling in tests without requiring subprocess patterns.
func Validate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [flags] <DAG definition>",
		Short: "Validate a DAG specification",
		Long: `Validate a DAG YAML file without executing it.

Prints a human-readable result instead of structured logs.
Checks structural correctness and references (e.g., step dependencies)
similar to the server-side spec validation.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := NewContext(cmd, nil)
			if err != nil {
				return fmt.Errorf("initialization error: %w", err)
			}
			return runValidate(ctx, args)
		},
	}

	// Initialize flags required by NewContext
	initFlags(cmd)

	return cmd
}

func runValidate(ctx *Context, args []string) error {
	// Try loading the DAG without evaluation, resolving relative names against DAGsDir
	dag, err := spec.Load(
		ctx,
		args[0],
		spec.WithoutEval(),
		spec.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	)

	if err != nil {
		// Collect and return a formatted error message
		return errors.New(formatValidationErrors(args[0], err))
	}

	// Run additional DAG-level validation (e.g., dependency references)
	if vErr := dag.Validate(); vErr != nil {
		return errors.New(formatValidationErrors(args[0], vErr))
	}

	// Success
	logger.Info(ctx, "DAG spec is valid", "file", args[0], "name", dag.GetName())
	return nil
}

// formatValidationErrors builds a readable error output from a (possibly wrapped) error.
func formatValidationErrors(file string, err error) string {
	// Collect message strings
	var msgs []string
	var list core.ErrorList
	if errors.As(err, &list) {
		msgs = list.ToStringList()
	} else {
		msgs = []string{err.Error()}
	}

	// Build readable, consistent output: one bullet per error, and if an
	// error spans multiple lines, indent subsequent lines for readability.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Validation failed for %s\n", file))
	for _, m := range msgs {
		lines := strings.Split(strings.TrimRight(m, "\n"), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if i == 0 {
				sb.WriteString("- ")
			} else {
				sb.WriteString("  ") // indent continuation lines of the same error
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
