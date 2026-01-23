package ssh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/pkg/sftp"
)

var _ executor.Executor = (*sftpExecutor)(nil)

type sftpExecutor struct {
	client      *Client
	direction   string // "upload" or "download"
	source      string
	destination string
	stdout      io.Writer
	stderr      io.Writer
}

// NewSFTPExecutor creates a new SFTP executor for file transfers.
func NewSFTPExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	var client *Client

	// Prefer step-level SSH configuration if present
	if len(step.ExecutorConfig.Config) > 0 {
		c, err := FromMapConfig(ctx, step.ExecutorConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to setup sftp executor: %w", err)
		}
		client = c
	} else if c := getSSHClientFromContext(ctx); c != nil {
		// Fall back to DAG-level SSH client from context
		client = c
	}

	if client == nil {
		return nil, fmt.Errorf("ssh configuration is not found for sftp executor")
	}

	// Get transfer parameters from config
	config := step.ExecutorConfig.Config
	direction, _ := config["direction"].(string)
	if direction == "" {
		direction = "upload" // Default to upload
	}
	if direction != "upload" && direction != "download" {
		return nil, fmt.Errorf("invalid direction %q: must be 'upload' or 'download'", direction)
	}

	source, _ := config["source"].(string)
	if source == "" {
		return nil, fmt.Errorf("source path is required for sftp executor")
	}

	destination, _ := config["destination"].(string)
	if destination == "" {
		return nil, fmt.Errorf("destination path is required for sftp executor")
	}

	return &sftpExecutor{
		client:      client,
		direction:   direction,
		source:      source,
		destination: destination,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
	}, nil
}

func (e *sftpExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *sftpExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *sftpExecutor) Kill(_ os.Signal) error {
	// SFTP operations are not interruptible in the same way as SSH commands.
	// The operation will complete or fail on its own.
	return nil
}

func (e *sftpExecutor) Run(ctx context.Context) error {
	conn, session, err := e.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()
	defer conn.Close()

	// Create SFTP client
	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	switch e.direction {
	case "upload":
		return e.upload(ctx, sftpClient)
	case "download":
		return e.download(ctx, sftpClient)
	default:
		return fmt.Errorf("unknown direction: %s", e.direction)
	}
}

// upload transfers files from local to remote.
func (e *sftpExecutor) upload(ctx context.Context, sftpClient *sftp.Client) error {
	// Check if source is a file or directory
	info, err := os.Stat(e.source)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", e.source, err)
	}

	if info.IsDir() {
		return e.uploadDir(ctx, sftpClient, e.source, e.destination)
	}
	return e.uploadFile(ctx, sftpClient, e.source, e.destination)
}

// uploadFile uploads a single file.
func (e *sftpExecutor) uploadFile(ctx context.Context, sftpClient *sftp.Client, localPath, remotePath string) error {
	logger.Info(ctx, "Uploading file",
		slog.String("source", localPath),
		slog.String("destination", remotePath),
	)

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file info for permissions
	info, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Ensure remote directory exists
	remoteDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
	}

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy contents
	bytes, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions
	if err := sftpClient.Chmod(remotePath, info.Mode()); err != nil {
		logger.Warn(ctx, "Failed to set remote file permissions", tag.Error(err))
	}

	fmt.Fprintf(e.stdout, "Uploaded %s (%d bytes) to %s\n", localPath, bytes, remotePath)
	return nil
}

// uploadDir uploads a directory recursively.
func (e *sftpExecutor) uploadDir(ctx context.Context, sftpClient *sftp.Client, localDir, remoteDir string) error {
	logger.Info(ctx, "Uploading directory",
		slog.String("source", localDir),
		slog.String("destination", remoteDir),
	)

	return filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		remotePath := filepath.Join(remoteDir, relPath)

		if info.IsDir() {
			// Create remote directory
			if err := sftpClient.MkdirAll(remotePath); err != nil {
				return fmt.Errorf("failed to create remote directory %s: %w", remotePath, err)
			}
			return nil
		}

		// Upload file
		return e.uploadFile(ctx, sftpClient, localPath, remotePath)
	})
}

// download transfers files from remote to local.
func (e *sftpExecutor) download(ctx context.Context, sftpClient *sftp.Client) error {
	// Check if source is a file or directory
	info, err := sftpClient.Stat(e.source)
	if err != nil {
		return fmt.Errorf("failed to stat remote %s: %w", e.source, err)
	}

	if info.IsDir() {
		return e.downloadDir(ctx, sftpClient, e.source, e.destination)
	}
	return e.downloadFile(ctx, sftpClient, e.source, e.destination)
}

// downloadFile downloads a single file.
func (e *sftpExecutor) downloadFile(ctx context.Context, sftpClient *sftp.Client, remotePath, localPath string) error {
	logger.Info(ctx, "Downloading file",
		slog.String("source", remotePath),
		slog.String("destination", localPath),
	)

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Get file info for permissions
	info, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %w", err)
	}

	// Ensure local directory exists
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("failed to create local directory %s: %w", localDir, err)
	}

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy contents
	bytes, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions
	if err := os.Chmod(localPath, info.Mode()); err != nil {
		logger.Warn(ctx, "Failed to set local file permissions", tag.Error(err))
	}

	fmt.Fprintf(e.stdout, "Downloaded %s (%d bytes) to %s\n", remotePath, bytes, localPath)
	return nil
}

// downloadDir downloads a directory recursively.
func (e *sftpExecutor) downloadDir(ctx context.Context, sftpClient *sftp.Client, remoteDir, localDir string) error {
	logger.Info(ctx, "Downloading directory",
		slog.String("source", remoteDir),
		slog.String("destination", localDir),
	)

	walker := sftpClient.Walk(remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return fmt.Errorf("error walking remote directory: %w", err)
		}

		remotePath := walker.Path()
		info := walker.Stat()

		// Calculate relative path
		relPath, err := filepath.Rel(remoteDir, remotePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		localPath := filepath.Join(localDir, relPath)

		if info.IsDir() {
			// Create local directory
			if err := os.MkdirAll(localPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create local directory %s: %w", localPath, err)
			}
			continue
		}

		// Download file
		if err := e.downloadFile(ctx, sftpClient, remotePath, localPath); err != nil {
			return err
		}
	}

	return nil
}

func init() {
	caps := core.ExecutorCapabilities{
		// SFTP executor doesn't use command/script - it uses source/destination paths
		Command:          false,
		MultipleCommands: false,
		Script:           false,
		Shell:            false,
	}
	executor.RegisterExecutor("sftp", NewSFTPExecutor, nil, caps)
}
