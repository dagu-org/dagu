package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dagu-org/dagu/internal/gitsync"
	"github.com/spf13/cobra"
)

// Sync returns the sync command with subcommands for Git sync operations.
func Sync() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Git sync operations",
		Long: `Manage Git synchronization for DAG definitions.

Git sync allows you to synchronize your DAG definitions with a remote Git repository.
This enables version control, collaboration, and backup of your workflow definitions.

Available Commands:
  status    Show the current sync status
  pull      Pull changes from remote repository
  publish   Publish local changes to remote repository
  discard   Discard local changes for a DAG`,
	}

	cmd.AddCommand(syncStatus())
	cmd.AddCommand(syncPull())
	cmd.AddCommand(syncPublish())
	cmd.AddCommand(syncDiscard())

	return cmd
}

// syncStatus shows the current Git sync status.
func syncStatus() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Show Git sync status",
			Long: `Display the current status of Git synchronization.

Shows overall sync status including:
- Whether sync is enabled
- Repository and branch information
- Last sync time and status
- Per-DAG status (synced, modified, untracked, conflict)

Example:
  dagu sync status`,
			Args: cobra.NoArgs,
		},
		nil,
		runSyncStatus,
	)
}

func runSyncStatus(ctx *Context, _ []string) error {
	syncSvc, err := newSyncService(ctx)
	if err != nil {
		return err
	}

	status, err := syncSvc.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get sync status: %w", err)
	}

	if !status.Enabled {
		fmt.Println("Git sync is not enabled")
		fmt.Println("\nTo enable Git sync, configure the gitSync section in your config file")
		return nil
	}

	// Print overall status
	fmt.Printf("Repository:  %s\n", status.Repository)
	fmt.Printf("Branch:      %s\n", status.Branch)
	fmt.Printf("Status:      %s\n", status.Summary)

	if status.LastSyncAt != nil {
		fmt.Printf("Last Sync:   %s\n", status.LastSyncAt.Format("2006-01-02 15:04:05"))
	}
	if status.LastSyncCommit != "" {
		fmt.Printf("Last Commit: %s\n", status.LastSyncCommit[:min(8, len(status.LastSyncCommit))])
	}
	if status.LastError != nil {
		fmt.Printf("Last Error:  %s\n", *status.LastError)
	}

	// Print counts
	fmt.Printf("\nDAG Status Counts:\n")
	fmt.Printf("  Synced:    %d\n", status.Counts.Synced)
	fmt.Printf("  Modified:  %d\n", status.Counts.Modified)
	fmt.Printf("  Untracked: %d\n", status.Counts.Untracked)
	fmt.Printf("  Conflict:  %d\n", status.Counts.Conflict)

	// Print per-DAG status if there are any non-synced DAGs
	if status.Counts.Modified > 0 || status.Counts.Untracked > 0 || status.Counts.Conflict > 0 {
		fmt.Printf("\nDAGs with pending changes:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  NAME\tSTATUS\tMODIFIED AT")
		for name, dagState := range status.DAGs {
			if dagState.Status != gitsync.StatusSynced {
				modifiedAt := "-"
				if dagState.ModifiedAt != nil {
					modifiedAt = dagState.ModifiedAt.Format("2006-01-02 15:04:05")
				}
				_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", name, dagState.Status, modifiedAt)
			}
		}
		_ = w.Flush()
	}

	return nil
}

// syncPull pulls changes from the remote repository.
func syncPull() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "pull",
			Short: "Pull changes from remote",
			Long: `Pull changes from the remote Git repository.

This command fetches and applies changes from the remote repository
to your local DAG definitions.

Example:
  dagu sync pull`,
			Args: cobra.NoArgs,
		},
		nil,
		runSyncPull,
	)
}

func runSyncPull(ctx *Context, _ []string) error {
	syncSvc, err := newSyncService(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Pulling changes from remote...")

	result, err := syncSvc.Pull(ctx)
	if err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}

	if result.Success {
		fmt.Println("Pull completed successfully")
		if len(result.Synced) > 0 {
			fmt.Printf("  Synced: %d DAGs\n", len(result.Synced))
		}
		if len(result.Modified) > 0 {
			fmt.Printf("  Modified: %d DAGs (local changes preserved)\n", len(result.Modified))
		}
		if len(result.Conflicts) > 0 {
			fmt.Printf("  Conflicts: %d DAGs (require resolution)\n", len(result.Conflicts))
		}
	} else {
		fmt.Printf("Pull completed with issues: %s\n", result.Message)
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s: %s\n", e.DAGID, e.Message)
		}
	}

	return nil
}

// syncPublish publishes local changes to the remote repository.
func syncPublish() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish [dag-name]",
		Short: "Publish changes to remote",
		Long: `Publish local changes to the remote Git repository.

When a DAG name is provided, only that DAG's changes are published.
When no DAG name is provided with --all flag, all modified DAGs are published.

Examples:
  dagu sync publish my_dag -m "Updated schedule"
  dagu sync publish --all -m "Batch update"`,
		Args: cobra.MaximumNArgs(1),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	cmd.Flags().Bool("all", false, "Publish all modified DAGs")
	cmd.Flags().BoolP("force", "f", false, "Force publish even with conflicts")

	return NewCommand(cmd, nil, runSyncPublish)
}

func runSyncPublish(ctx *Context, args []string) error {
	syncSvc, err := newSyncService(ctx)
	if err != nil {
		return err
	}

	message, _ := ctx.Command.Flags().GetString("message")
	publishAll, _ := ctx.Command.Flags().GetBool("all")
	force, _ := ctx.Command.Flags().GetBool("force")

	if len(args) == 0 && !publishAll {
		return fmt.Errorf("either provide a DAG name or use --all flag")
	}

	if len(args) > 0 && publishAll {
		return fmt.Errorf("cannot specify both a DAG name and --all flag")
	}

	var result *gitsync.SyncResult

	if publishAll {
		fmt.Println("Publishing all modified DAGs...")
		result, err = syncSvc.PublishAll(ctx, message)
	} else {
		fmt.Printf("Publishing DAG: %s...\n", args[0])
		result, err = syncSvc.Publish(ctx, args[0], message, force)
	}

	if err != nil {
		if gitsync.IsConflict(err) {
			fmt.Println("Conflict detected!")
			fmt.Println("The DAG has been modified on the remote since your last sync.")
			fmt.Println("Use --force to overwrite remote changes, or pull first to resolve.")
			return err
		}
		return fmt.Errorf("failed to publish: %w", err)
	}

	if result.Success {
		fmt.Println("Publish completed successfully")
		if len(result.Synced) > 0 {
			fmt.Printf("  Published: %d DAGs\n", len(result.Synced))
		}
	} else {
		fmt.Printf("Publish completed with issues: %s\n", result.Message)
	}

	if len(result.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s: %s\n", e.DAGID, e.Message)
		}
	}

	return nil
}

// syncDiscard discards local changes for a DAG.
func syncDiscard() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "discard <dag-name>",
			Short: "Discard local changes",
			Long: `Discard local changes for a DAG and restore the remote version.

This command reverts local modifications to a DAG, restoring it to
the version from the remote repository.

WARNING: This will permanently discard your local changes!

Example:
  dagu sync discard my_dag`,
			Args: cobra.ExactArgs(1),
		},
		nil,
		runSyncDiscard,
	)
}

func runSyncDiscard(ctx *Context, args []string) error {
	syncSvc, err := newSyncService(ctx)
	if err != nil {
		return err
	}

	dagName := args[0]

	fmt.Printf("Discarding local changes for DAG: %s\n", dagName)
	fmt.Println("WARNING: This will permanently discard your local changes!")

	if err := syncSvc.Discard(ctx, dagName); err != nil {
		if gitsync.IsDAGNotFound(err) {
			return fmt.Errorf("DAG not found: %s", dagName)
		}
		return fmt.Errorf("failed to discard changes: %w", err)
	}

	fmt.Println("Changes discarded successfully")
	return nil
}

// newSyncService creates a new GitSync service from the context configuration.
func newSyncService(ctx *Context) (gitsync.Service, error) {
	cfg := ctx.Config.GitSync

	syncCfg := gitsync.NewConfigFromGlobal(cfg)

	if !syncCfg.Enabled {
		return nil, fmt.Errorf("git sync is not enabled, set gitSync.enabled=true in your config")
	}

	// Create the service
	svc := gitsync.NewService(syncCfg, ctx.Config.Paths.DAGsDir, ctx.Config.Paths.DataDir)
	return svc, nil
}
