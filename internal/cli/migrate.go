package cli

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/migration"
	"github.com/spf13/cobra"
)

// Migrate creates the migrate command with subcommands
func Migrate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate legacy data to new format",
		Long: `Migrate various types of legacy data to new formats.

Available subcommands:
  history - Migrate DAG run history from v1.16 format to v1.17+ format`,
	}

	cmd.AddCommand(MigrateHistoryCommand())
	return cmd
}

// MigrateHistoryCommand creates a command to migrate history data
func MigrateHistoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Migrate legacy history data",
		Long: `Migrate DAG run history from the legacy format (v1.16 and earlier) to the new format (v1.17+).

This command will:
- Detect if legacy history data exists
- Convert and migrate all historical DAG runs to the new format
- Archive the old data to history_migrated_<timestamp> directory
- Report migration progress and any errors

Example:
  dagu migrate history`,
	}

	return NewCommand(cmd, nil, func(ctx *Context, _ []string) error {
		return runMigration(ctx)
	})
}

func runMigration(ctx *Context) error {
	logger.Info(ctx.Context, "Starting history migration")

	// Create DAG store for loading DAG definitions
	dagStore, err := ctx.dagStore(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create DAG store: %w", err)
	}

	// Create migrator
	migrator := migration.NewHistoryMigrator(
		ctx.DAGRunStore,
		dagStore,
		ctx.Config.Paths.DataDir,
		ctx.Config.Paths.DAGsDir,
	)

	// Check if migration is needed
	needsMigration, err := migrator.NeedsMigration(ctx.Context)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if !needsMigration {
		logger.Info(ctx.Context, "No legacy history data found, migration not needed")
		return nil
	}

	// Run migration
	result, err := migrator.Migrate(ctx.Context)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Report results
	if len(result.Errors) > 0 {
		logger.Warn(ctx.Context, "Migration completed with errors",
			"total_dags", result.TotalDAGs,
			"total_runs", result.TotalRuns,
			"migrated", result.MigratedRuns,
			"skipped", result.SkippedRuns,
			"failed", result.FailedRuns,
		)
		for i, err := range result.Errors {
			if i < 10 { // Limit error output
				logger.Error(ctx.Context, "Migration error", "err", err)
			}
		}
		if len(result.Errors) > 10 {
			logger.Warn(ctx.Context, "Additional errors omitted", "count", len(result.Errors)-10)
		}
	} else {
		logger.Info(ctx.Context, "Migration completed 🎉",
			"total_dags", result.TotalDAGs,
			"total_runs", result.TotalRuns,
			"migrated", result.MigratedRuns,
			"skipped", result.SkippedRuns,
			"failed", result.FailedRuns,
		)
	}

	// Move legacy data to archive if migration was successful
	if result.FailedRuns == 0 {
		if err := migrator.MoveLegacyData(ctx.Context); err != nil {
			logger.Error(ctx.Context, "Failed to move legacy data to archive", "err", err)
			logger.Info(ctx.Context, "Legacy data remains in original location", "path", filepath.Join(ctx.Config.Paths.DataDir, "history"))
		}
	} else {
		logger.Info(ctx.Context, "Legacy data not moved due to migration errors", "failed_runs", result.FailedRuns)
		logger.Info(ctx.Context, "Fix errors and run migration again to complete the process")
	}

	return nil
}
