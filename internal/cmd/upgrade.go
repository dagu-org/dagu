package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/upgrade"
	"github.com/spf13/cobra"
)

// Upgrade command flags
var upgradeFlags = []commandLineFlag{
	{name: "check", usage: "Only check if an update is available", isBool: true},
	{name: "version", shorthand: "v", usage: "Upgrade to specific version (e.g., v1.30.0)"},
	{name: "dry-run", usage: "Show what would happen without making changes", isBool: true},
	{name: "backup", usage: "Create backup of current binary before upgrade", isBool: true},
	yesFlag,
	{name: "pre-release", usage: "Include pre-release versions", isBool: true},
}

// Upgrade returns the upgrade command.
func Upgrade() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "upgrade [flags]",
			Short: "Upgrade dagu to the latest version",
			Long: `Upgrade the dagu binary to the latest version or a specified version.

This command downloads the latest (or specified) release from GitHub, verifies
the checksum, and replaces the current binary.

Examples:
  dagu upgrade                    # Upgrade to latest version
  dagu upgrade --check            # Check if an update is available
  dagu upgrade --version v1.30.0  # Upgrade to specific version
  dagu upgrade --dry-run          # Show what would happen
  dagu upgrade --backup           # Create backup before upgrade
  dagu upgrade -y                 # Skip confirmation prompt

Note: This command cannot be used if dagu was installed via Homebrew, Snap, or
is running in Docker. Use the appropriate package manager instead.`,
		},
		upgradeFlags,
		runUpgrade,
	)
}

func runUpgrade(ctx *Context, _ []string) error {
	// Parse flags
	checkOnly, err := ctx.Command.Flags().GetBool("check")
	if err != nil {
		return fmt.Errorf("failed to get check flag: %w", err)
	}

	targetVersion, err := ctx.StringParam("version")
	if err != nil {
		return fmt.Errorf("failed to get version flag: %w", err)
	}

	dryRun, err := ctx.Command.Flags().GetBool("dry-run")
	if err != nil {
		return fmt.Errorf("failed to get dry-run flag: %w", err)
	}

	createBackup, err := ctx.Command.Flags().GetBool("backup")
	if err != nil {
		return fmt.Errorf("failed to get backup flag: %w", err)
	}

	skipConfirm, err := ctx.Command.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get yes flag: %w", err)
	}

	includePreRelease, err := ctx.Command.Flags().GetBool("pre-release")
	if err != nil {
		return fmt.Errorf("failed to get pre-release flag: %w", err)
	}

	// Check if self-upgrade is possible
	canUpgrade, reason := upgrade.CanSelfUpgrade()
	if !canUpgrade {
		return fmt.Errorf("%s", reason)
	}

	opts := upgrade.Options{
		TargetVersion:     targetVersion,
		CheckOnly:         checkOnly,
		DryRun:            dryRun,
		CreateBackup:      createBackup,
		Force:             skipConfirm,
		IncludePreRelease: includePreRelease,
	}

	// If check-only, just show the result
	if checkOnly {
		result, err := upgrade.Upgrade(ctx, opts)
		if err != nil {
			return err
		}
		fmt.Print(upgrade.FormatCheckResult(result))
		return nil
	}

	// First, check what will happen
	checkOpts := opts
	checkOpts.DryRun = true
	result, err := upgrade.Upgrade(ctx, checkOpts)
	if err != nil {
		return err
	}

	// If already on latest and not forced, exit early
	if !result.UpgradeNeeded && !skipConfirm {
		fmt.Println("Already running the latest version.")
		return nil
	}

	// Show what will happen
	if dryRun {
		fmt.Print(upgrade.FormatResult(result))
		return nil
	}

	// Confirmation prompt (unless skipped with -y)
	if !skipConfirm {
		fmt.Printf("Current version: %s\n", result.CurrentVersion)
		fmt.Printf("Target version:  %s\n\n", result.TargetVersion)
		fmt.Println("The following changes will be made:")
		fmt.Printf("  - Download: %s (%s)\n", result.AssetName, upgrade.FormatBytes(result.AssetSize))
		fmt.Println("  - Verify:   SHA256 checksum")
		fmt.Printf("  - Replace:  %s\n\n", result.ExecutablePath)

		if !confirmAction("Continue?") {
			fmt.Println("Upgrade cancelled.")
			return nil
		}
	}

	// Perform the actual upgrade
	fmt.Println("Downloading...")
	result, err = upgrade.Upgrade(ctx, opts)
	if err != nil {
		return err
	}

	if result.WasUpgraded {
		fmt.Printf("\nSuccessfully upgraded to %s\n", result.TargetVersion)
		if result.BackupPath != "" {
			fmt.Printf("Backup created: %s\n", result.BackupPath)
		}
	}

	return nil
}

// confirmAction prompts the user for confirmation.
func confirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
