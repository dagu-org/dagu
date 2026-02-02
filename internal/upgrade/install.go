package upgrade

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mholt/archives"
)

// InstallOptions configures the installation operation.
type InstallOptions struct {
	ArchivePath  string // Downloaded .tar.gz
	TargetPath   string // Path to current dagu binary
	CreateBackup bool
}

// InstallResult contains information about the installation.
type InstallResult struct {
	BackupPath string
	Installed  bool
}

// Install extracts the binary from the archive and replaces the current binary.
func Install(ctx context.Context, opts InstallOptions) (*InstallResult, error) {
	result := &InstallResult{}

	// Extract archive to a temporary directory
	tempDir, err := os.MkdirTemp("", "dagu-upgrade-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the archive
	if err := extractArchive(ctx, opts.ArchivePath, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	// Find the dagu binary in extracted files
	binaryName := "dagu"
	if runtime.GOOS == "windows" {
		binaryName = "dagu.exe"
	}

	extractedBinary, err := findBinary(tempDir, binaryName)
	if err != nil {
		return nil, err
	}

	// Create backup if requested
	if opts.CreateBackup {
		backupPath := opts.TargetPath + ".bak"
		if err := copyFile(opts.TargetPath, backupPath); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
		result.BackupPath = backupPath
	}

	// Replace the binary atomically
	if err := replaceBinary(extractedBinary, opts.TargetPath); err != nil {
		// Try to restore from backup if we created one
		if result.BackupPath != "" {
			_ = copyFile(result.BackupPath, opts.TargetPath)
		}
		return nil, fmt.Errorf("failed to replace binary: %w", err)
	}

	result.Installed = true
	return result, nil
}

// extractArchive extracts a tar.gz archive to the destination directory.
func extractArchive(ctx context.Context, archivePath, destDir string) error {
	srcFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer srcFile.Close()

	// Identify the archive format
	format, _, err := archives.Identify(ctx, filepath.Base(archivePath), srcFile)
	if err != nil {
		return fmt.Errorf("failed to identify archive format: %w", err)
	}

	// Reset file position after Identify
	if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset file position: %w", err)
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("archive format does not support extraction")
	}

	// Extract files
	err = extractor.Extract(ctx, srcFile, func(ctx context.Context, f archives.FileInfo) error {
		if f.IsDir() {
			return nil
		}

		// Security: prevent path traversal
		name := filepath.Clean(f.NameInArchive)
		if strings.HasPrefix(name, "..") {
			return fmt.Errorf("invalid path in archive: %s", f.NameInArchive)
		}

		targetPath := filepath.Join(destDir, name)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Extract file
		src, err := f.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		return err
	})

	return err
}

// findBinary searches for the binary in the extracted directory.
func findBinary(dir, binaryName string) (string, error) {
	var found string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if d.Name() == binaryName {
			found = path
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error searching for binary: %w", err)
	}

	if found == "" {
		return "", fmt.Errorf("binary %s not found in archive", binaryName)
	}

	return found, nil
}

// replaceBinary atomically replaces the target binary with the source.
func replaceBinary(src, target string) error {
	// Get the permissions of the target file (if it exists)
	perm := os.FileMode(0755)
	if info, err := os.Stat(target); err == nil {
		perm = info.Mode().Perm()
	}

	// On Unix: os.Rename is atomic on same filesystem
	// On Windows: we need a different approach

	if runtime.GOOS == "windows" {
		return replaceWindowsBinary(src, target, perm)
	}

	return replaceUnixBinary(src, target, perm)
}

// replaceUnixBinary replaces the binary on Unix systems.
func replaceUnixBinary(src, target string, perm os.FileMode) error {
	// Create a temp file in the same directory as target
	dir := filepath.Dir(target)
	tempFile, err := os.CreateTemp(dir, "dagu-new-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	// Copy source to temp file
	if err := copyFile(src, tempPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Set permissions
	if err := os.Chmod(tempPath, perm); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, target); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// replaceWindowsBinary replaces the binary on Windows systems.
func replaceWindowsBinary(src, target string, perm os.FileMode) error {
	// On Windows, we can't rename over a running binary
	// So we rename the old one first, then copy the new one

	oldPath := target + ".old"

	// Remove any existing .old file
	_ = os.Remove(oldPath)

	// Rename current binary to .old
	if err := os.Rename(target, oldPath); err != nil {
		// If target doesn't exist, that's fine
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to rename old binary: %w", err)
		}
	}

	// Copy new binary to target
	if err := copyFile(src, target); err != nil {
		// Try to restore old binary
		_ = os.Rename(oldPath, target)
		return err
	}

	// Chmod may fail on Windows - ignore error
	_ = os.Chmod(target, perm)

	// Remove old binary (will fail if it's running, but that's okay)
	_ = os.Remove(oldPath)

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// GetExecutablePath returns the path to the currently running binary.
func GetExecutablePath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	return execPath, nil
}
