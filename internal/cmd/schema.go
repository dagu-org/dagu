// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"

	"github.com/dagucloud/dagu/internal/agent/schema"
	"github.com/spf13/cobra"
)

// Schema creates the 'schema' CLI command that displays JSON schema
// documentation for DAG definitions or configuration.
func Schema() *cobra.Command {
	return &cobra.Command{
		Use:   "schema <dag|config> [path]",
		Short: "Display schema documentation for DAG or config",
		Long: `Browse the JSON schema for DAG definitions or configuration.

Available schemas: dag, config

Call without a path to see all root-level fields. Use a dot-separated
path to drill into nested sections.`,
		Example: `  dagu schema dag                  Show all DAG root-level fields
  dagu schema dag steps            Show step properties
  dagu schema dag steps.container  Show container configuration
  dagu schema config               Show all config root-level fields
  dagu schema config server        Show server configuration`,
		ValidArgs: []string{"dag", "config"},
		Args:      cobra.RangeArgs(1, 2),
		RunE:      runSchema,
	}
}

func runSchema(cmd *cobra.Command, args []string) error {
	schemaName := args[0]
	if schemaName == "help" {
		return cmd.Help()
	}
	var path string
	if len(args) > 1 {
		path = args[1]
	}

	result, err := schema.DefaultRegistry.NavigateFull(schemaName, path)
	if err != nil {
		return fmt.Errorf("schema navigation failed: %w", err)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), result)
	return nil
}
