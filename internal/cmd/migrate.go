package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/migration"
	"github.com/dagu-org/dagu/internal/persistence/filedag"
	"github.com/spf13/cobra"
)

// CmdMigrate creates the migrate command with subcommands
func CmdMigrate() *cobra.Command {
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
- Create a backup of the legacy data (unless --skip-backup is used)
- Convert and migrate all historical DAG runs to the new format
- Report migration progress and any errors

Example:
  dagu migrate history
  dagu migrate history --skip-backup`,
	}
	
	var skipBackup bool
	cmd.Flags().BoolVar(&skipBackup, "skip-backup", false, "Skip creating backup of legacy data")
	
	return NewCommand(cmd, nil, func(ctx *Context, args []string) error {
		return runMigration(ctx, skipBackup)
	})
}

func runMigration(ctx *Context, skipBackup bool) error {
	logger.Info(ctx.Context, "Starting history migration")
	
	// Create DAG store for loading DAG definitions
	dagStore, err := ctx.dagStore(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create DAG store: %w", err)
	}
	
	// Create migrator
	migrator := migration.NewHistoryMigrator(
		ctx.LegacyStore,
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
	logger.Info(ctx.Context, "Migration completed",
		"total_dags", result.TotalDAGs,
		"total_runs", result.TotalRuns,
		"migrated", result.MigratedRuns,
		"skipped", result.SkippedRuns,
		"failed", result.FailedRuns,
	)
	
	if len(result.Errors) > 0 {
		logger.Warn(ctx.Context, "Migration completed with errors", "error_count", len(result.Errors))
		for i, err := range result.Errors {
			if i < 10 { // Limit error output
				logger.Error(ctx.Context, "Migration error", "error", err)
			}
		}
		if len(result.Errors) > 10 {
			logger.Warn(ctx.Context, "Additional errors omitted", "count", len(result.Errors)-10)
		}
	}
	
	// Move legacy data to archive if migration was successful
	if result.FailedRuns == 0 || skipBackup {
		if err := migrator.MoveLegacyData(ctx.Context); err != nil {
			logger.Error(ctx.Context, "Failed to move legacy data to archive", "error", err)
			logger.Info(ctx.Context, "Legacy data remains in original location", "path", filepath.Join(ctx.Config.Paths.DataDir, "history"))
		}
	} else {
		logger.Info(ctx.Context, "Legacy data not moved due to migration errors", "failed_runs", result.FailedRuns)
		logger.Info(ctx.Context, "Fix errors and run migration again to complete the process")
	}
	
	return nil
}

