package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/config"
)

// Options configures the upgrade operation.
type Options struct {
	TargetVersion     string // Empty = latest
	CheckOnly         bool
	DryRun            bool
	CreateBackup      bool
	Force             bool
	IncludePreRelease bool
}

// Result contains the result of an upgrade operation.
type Result struct {
	CurrentVersion string
	TargetVersion  string
	UpgradeNeeded  bool
	WasUpgraded    bool
	BackupPath     string
	DryRun         bool
	AssetName      string
	AssetSize      int64
	DownloadURL    string
	ExecutablePath string
}

// InstallMethod represents how dagu was installed.
type InstallMethod int

const (
	InstallMethodUnknown InstallMethod = iota
	InstallMethodBinary
	InstallMethodHomebrew
	InstallMethodSnap
	InstallMethodDocker
	InstallMethodGoInstall
)

// String returns a human-readable name for the install method.
func (m InstallMethod) String() string {
	switch m {
	case InstallMethodUnknown:
		return "unknown"
	case InstallMethodBinary:
		return "binary"
	case InstallMethodHomebrew:
		return "homebrew"
	case InstallMethodSnap:
		return "snap"
	case InstallMethodDocker:
		return "docker"
	case InstallMethodGoInstall:
		return "go install"
	}
	return "unknown"
}

// DetectInstallMethod checks how dagu was installed.
func DetectInstallMethod() InstallMethod {
	execPath, err := GetExecutablePath()
	if err != nil {
		return InstallMethodUnknown
	}

	// Homebrew: path contains "/Cellar/" or "/homebrew/"
	if strings.Contains(execPath, "/Cellar/") || strings.Contains(execPath, "/homebrew/") {
		return InstallMethodHomebrew
	}

	// Snap: path starts with "/snap/" or SNAP env var exists
	if strings.HasPrefix(execPath, "/snap/") || os.Getenv("SNAP") != "" {
		return InstallMethodSnap
	}

	// Docker: /.dockerenv exists
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return InstallMethodDocker
	}

	// Go install: path in GOPATH/bin or GOBIN
	gopath := os.Getenv("GOPATH")
	gobin := os.Getenv("GOBIN")
	if gobin != "" && strings.HasPrefix(execPath, gobin) {
		return InstallMethodGoInstall
	}
	if gopath != "" && strings.HasPrefix(execPath, filepath.Join(gopath, "bin")) {
		return InstallMethodGoInstall
	}

	return InstallMethodBinary
}

// CanSelfUpgrade returns true if dagu can perform a self-upgrade.
func CanSelfUpgrade() (bool, string) {
	method := DetectInstallMethod()
	switch method {
	case InstallMethodHomebrew:
		return false, "Installed via Homebrew. Use 'brew upgrade dagu' instead."
	case InstallMethodSnap:
		return false, "Installed via Snap. Use 'snap refresh dagu' instead."
	case InstallMethodDocker:
		return false, "Running in Docker. Pull the latest image instead."
	case InstallMethodUnknown, InstallMethodBinary, InstallMethodGoInstall:
		return true, ""
	}
	return true, ""
}

// Upgrade performs the upgrade operation.
func Upgrade(ctx context.Context, opts Options) (*Result, error) {
	result := &Result{
		CurrentVersion: config.Version,
		DryRun:         opts.DryRun,
	}

	// Check if we can self-upgrade
	canUpgrade, reason := CanSelfUpgrade()
	if !canUpgrade {
		return nil, fmt.Errorf("%s", reason)
	}

	// Get executable path
	execPath, err := GetExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	result.ExecutablePath = execPath

	// Check platform support
	platform := Detect()
	if !platform.IsSupported() {
		return nil, fmt.Errorf("platform %s is not supported\n%s", platform, SupportedPlatformsMessage())
	}

	// Parse current version
	currentV, err := ParseVersion(config.Version)
	if err != nil {
		return nil, fmt.Errorf("cannot determine current version (development build?): %w", err)
	}

	// Create GitHub client
	client := NewGitHubClient()

	// Fetch target release
	var release *Release
	if opts.TargetVersion != "" {
		release, err = client.GetRelease(ctx, opts.TargetVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch release %s: %w", opts.TargetVersion, err)
		}
	} else {
		release, err = client.GetLatestRelease(ctx, opts.IncludePreRelease)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest release: %w", err)
		}
	}

	result.TargetVersion = release.TagName

	// Parse target version
	targetV, err := ParseVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target version: %w", err)
	}

	// Compare versions
	result.UpgradeNeeded = IsNewer(currentV, targetV)

	// If check-only, return result now
	if opts.CheckOnly {
		return result, nil
	}

	// If not an upgrade (same or older), exit unless forced
	if !result.UpgradeNeeded && !opts.Force {
		return result, nil
	}

	// Find asset for current platform
	asset, err := FindAsset(release, platform, release.TagName)
	if err != nil {
		return nil, err
	}

	result.AssetName = asset.Name
	result.AssetSize = asset.Size
	result.DownloadURL = asset.BrowserDownloadURL

	// Get checksums
	checksums, err := client.GetChecksums(ctx, release)
	if err != nil {
		return nil, fmt.Errorf("failed to get checksums: %w", err)
	}

	expectedHash, ok := checksums[asset.Name]
	if !ok {
		return nil, fmt.Errorf("checksum for %s not found", asset.Name)
	}

	// If dry-run, return plan without making changes
	if opts.DryRun {
		return result, nil
	}

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "dagu-upgrade-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, asset.Name)

	// Download archive
	if err := Download(ctx, DownloadOptions{
		URL:          asset.BrowserDownloadURL,
		Destination:  archivePath,
		ExpectedHash: expectedHash,
	}); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Install (extract + replace)
	installResult, err := Install(ctx, InstallOptions{
		ArchivePath:  archivePath,
		TargetPath:   execPath,
		CreateBackup: opts.CreateBackup,
	})
	if err != nil {
		return nil, fmt.Errorf("installation failed: %w", err)
	}

	result.WasUpgraded = installResult.Installed
	result.BackupPath = installResult.BackupPath

	// Update cache with new version info
	_ = SaveCache(&UpgradeCheckCache{
		CurrentVersion:  release.TagName,
		LatestVersion:   release.TagName,
		UpdateAvailable: false,
	})

	return result, nil
}

// FormatResult formats the upgrade result for display.
func FormatResult(r *Result) string {
	var sb strings.Builder

	if r.DryRun {
		sb.WriteString("Dry run - no changes will be made\n\n")
	}

	fmt.Fprintf(&sb, "Current version: %s\n", r.CurrentVersion)
	fmt.Fprintf(&sb, "Target version:  %s\n", r.TargetVersion)

	if !r.UpgradeNeeded && !r.WasUpgraded {
		sb.WriteString("\nAlready running the latest version.\n")
		return sb.String()
	}

	if r.DryRun {
		sb.WriteString("\nThe following changes will be made:\n")
		fmt.Fprintf(&sb, "  - Download: %s (%s)\n", r.AssetName, FormatBytes(r.AssetSize))
		sb.WriteString("  - Verify:   SHA256 checksum\n")
		fmt.Fprintf(&sb, "  - Replace:  %s\n", r.ExecutablePath)
		return sb.String()
	}

	if r.WasUpgraded {
		sb.WriteString("\nUpgrade successful!\n")
		if r.BackupPath != "" {
			fmt.Fprintf(&sb, "Backup created: %s\n", r.BackupPath)
		}
	}

	return sb.String()
}

// FormatCheckResult formats the check result for display.
func FormatCheckResult(r *Result) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Current version: %s\n", r.CurrentVersion)
	fmt.Fprintf(&sb, "Latest version:  %s\n", r.TargetVersion)

	if r.UpgradeNeeded {
		sb.WriteString("\nAn update is available. Run 'dagu upgrade' to update.\n")
	} else {
		sb.WriteString("\nYou are running the latest version.\n")
	}

	return sb.String()
}
