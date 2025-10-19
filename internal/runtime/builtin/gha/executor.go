package gha

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	"github.com/sirupsen/logrus"
)

// githubAction executor runs a GitHub Action locally using nektos/act
/* Example DAG:
```yaml
steps:
  - name: checkout
    uses: actions/checkout@v4
    with:
      repository: myorg/myrepo
      ref: main

  - name: setup-go
    uses: actions/setup-go@v5
    with:
      go-version: '1.21'
```
*/

var _ executor.Executor = (*githubAction)(nil)

type githubAction struct {
	step   core.Step
	stdout io.Writer
	stderr io.Writer
	cancel func()
}

// daguJobLoggerFactory implements runner.JobLoggerFactory to integrate
// act's logging with Dagu's stderr writer without hijacking global stdout/stderr
type daguJobLoggerFactory struct {
	stderr io.Writer
}

// WithJobLogger creates a logrus logger that writes to Dagu's stderr
func (f *daguJobLoggerFactory) WithJobLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(f.stderr) // Logs go to stderr
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:    false,
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05",
		DisableTimestamp: false,
	})
	return logger
}

func (e *githubAction) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *githubAction) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *githubAction) Kill(sig os.Signal) error {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	return nil
}

func (e *githubAction) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	e.cancel = cancelFunc
	defer cancelFunc()

	// Determine the actual working directory where files should be checked out
	// This is where the action will run and where files will persist
	actualWorkDir := e.step.Dir
	if actualWorkDir == "" {
		// If no dir specified, use current working directory
		actualWorkDir, _ = os.Getwd()
	}

	// Ensure the working directory exists
	if err := os.MkdirAll(actualWorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Create temporary directory for workflow file (not for execution)
	tmpDir, err := os.MkdirTemp("", "dagu-github-action-workflow-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate GitHub Actions workflow YAML
	workflowYAML, err := e.generateWorkflowYAML()
	if err != nil {
		return fmt.Errorf("failed to generate workflow YAML: %w", err)
	}

	// Write workflow file to temp directory
	workflowDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	workflowFile := filepath.Join(workflowDir, "workflow.yml")
	if err := os.WriteFile(workflowFile, []byte(workflowYAML), 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	// Run the action using act library, using actualWorkDir as the workspace
	return e.executeAct(ctx, actualWorkDir, workflowFile)
}

func (e *githubAction) generateWorkflowYAML() (string, error) {
	if e.step.ExecutorConfig.Config == nil || e.step.ExecutorConfig.Config["action"] == nil {
		return "", fmt.Errorf("uses field is required for GitHub Action executor")
	}

	// Copy with parameters to avoid modifying original
	withParams := make(map[string]string)
	for k, v := range e.step.ExecutorConfig.Config {
		withParams[k] = fmt.Sprintf("%v", v)
	}

	// Special handling: checkout action requires token input
	// Auto-inject if not provided by user
	action := fmt.Sprintf("%v", e.step.ExecutorConfig.Config["action"])
	if action == "actions/checkout@v4" || action == "actions/checkout@v3" {
		if _, hasToken := withParams["token"]; !hasToken {
			// Use empty string to make the action use default unauthenticated access
			// This works for public repos
			withParams["token"] = ""
		}
	}

	// Build the with section
	withSection := ""
	if len(withParams) > 0 {
		withSection = "\n        with:\n"
		for key, value := range withParams {
			// Quote the value if it contains special characters or is empty
			if value == "" || value == "${{ github.token }}" {
				withSection += fmt.Sprintf("          %s: '%s'\n", key, value)
			} else {
				withSection += fmt.Sprintf("          %s: %s\n", key, value)
			}
		}
	}

	// Generate workflow YAML
	yaml := fmt.Sprintf(`name: Dagu GitHub Action
on: push
jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - uses: %s%s`,
		action,
		withSection,
	)

	return yaml, nil
}

func (e *githubAction) executeAct(ctx context.Context, workDir, workflowFile string) error {
	// Open and read the workflow file
	file, err := os.Open(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to open workflow file: %w", err)
	}
	defer file.Close()

	// Create workflow planner
	planner, err := model.NewSingleWorkflowPlanner("dagu-action", file)
	if err != nil {
		return fmt.Errorf("failed to create workflow planner: %w", err)
	}

	// Get GitHub token from environment (optional)
	// Only use if actually set - don't use dummy tokens as they cause auth failures
	token := os.Getenv("GITHUB_TOKEN")

	// Create act runner config with volume binding
	// workDir is the actual working directory where files will be checked out
	config := &runner.Config{
		Workdir:        workDir,
		BindWorkdir:    true, // Bind the workdir to the container so files persist on host
		EventName:      "push",
		GitHubInstance: "github.com", // Configure GitHub instance for action resolution
		Platforms: map[string]string{
			"ubuntu-latest": "node:20-bullseye", // Use a lightweight Node.js image for ubuntu-latest
		},
	}

	// Only set token if one is actually provided (for private repos/actions)
	if token != "" {
		config.Token = token
	}

	// Inject custom JobLoggerFactory into context
	// This makes act write logs to e.stderr without hijacking global stdout/stderr
	loggerFactory := &daguJobLoggerFactory{
		stderr: e.stderr,
	}
	ctx = runner.WithJobLoggerFactory(ctx, loggerFactory)

	// Create runner
	r, err := runner.New(config)
	if err != nil {
		return fmt.Errorf("failed to create act runner: %w", err)
	}

	// Get the plan for the event
	plan, err := planner.PlanEvent("push")
	if err != nil {
		return fmt.Errorf("failed to plan workflow: %w", err)
	}

	// Execute the plan
	// Logs will go to stderr via our JobLoggerFactory
	// stdout is reserved for capturing action outputs
	executor := r.NewPlanExecutor(plan)
	if err := executor(ctx); err != nil {
		return fmt.Errorf("failed to execute GitHub Action: %w", err)
	}

	return nil
}

func newGitHubAction(ctx context.Context, step core.Step) (executor.Executor, error) {
	if step.ExecutorConfig.Config == nil || step.ExecutorConfig.Config["action"] == nil {
		return nil, fmt.Errorf("uses field is required for GitHub Action executor")
	}
	action := fmt.Sprintf("%v", step.ExecutorConfig.Config["action"])
	if action == "" {
		return nil, fmt.Errorf("uses field is required for GitHub Action executor")
	}

	return &githubAction{
		step:   step,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}, nil
}

func init() {
	executor.RegisterExecutor("github-action", newGitHubAction, nil)
}
