package gha

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/goccy/go-yaml"
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
// act's logging with Dagu's stdout/stderr writers without hijacking global stdout/stderr
type daguJobLoggerFactory struct {
	stdout io.Writer
	stderr io.Writer
}

// daguLogrusHook intercepts log entries and routes them based on content
// Raw output (container stdout/stderr) goes to stdout, other logs go to stderr
type daguLogrusHook struct {
	stdout io.Writer
	stderr io.Writer
}

func (h *daguLogrusHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *daguLogrusHook) Fire(entry *logrus.Entry) error {
	// Check if this is raw output from the container
	if rawOutput, ok := entry.Data["raw_output"]; ok && rawOutput == true {
		// Container output goes to stdout - write only the message, not formatted log entry
		_, err := h.stdout.Write([]byte(entry.Message))
		return err
	}
	// All other logs go to stderr
	return nil
}

// WithJobLogger creates a logrus logger that routes output appropriately
func (f *daguJobLoggerFactory) WithJobLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(f.stderr) // Default output to stderr
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:    false,
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05",
		DisableTimestamp: false,
	})
	// Add hook to intercept raw_output and send to stdout
	logger.AddHook(&daguLogrusHook{
		stdout: f.stdout,
		stderr: f.stderr,
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

// workflowDefinition represents a GitHub Actions workflow
type workflowDefinition struct {
	Name string                 `yaml:"name"`
	On   string                 `yaml:"on"`
	Jobs map[string]jobDefinition `yaml:"jobs"`
}

type jobDefinition struct {
	RunsOn string           `yaml:"runs-on"`
	Steps  []stepDefinition `yaml:"steps"`
}

type stepDefinition struct {
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with,omitempty"`
}

func (e *githubAction) generateWorkflowYAML() (string, error) {
	if e.step.ExecutorConfig.Config == nil || e.step.ExecutorConfig.Config["action"] == nil {
		return "", fmt.Errorf("uses field is required for GitHub Action executor")
	}

	action := fmt.Sprintf("%v", e.step.ExecutorConfig.Config["action"])
	action = strings.TrimSpace(action)

	// Copy with parameters, excluding Dagu-specific config keys
	withParams := make(map[string]any)
	for k, v := range e.step.ExecutorConfig.Config {
		// Skip Dagu-specific config keys that shouldn't go to the action's 'with:'
		if k == "action" || k == "runner" {
			continue
		}
		withParams[k] = v
	}

	// Special handling: checkout action requires token input
	// Auto-inject if not provided by user
	if action == "actions/checkout@v4" || action == "actions/checkout@v3" {
		if _, hasToken := withParams["token"]; !hasToken {
			// Use empty string to make the action use default unauthenticated access
			// This works for public repos
			withParams["token"] = ""
		}
	}

	// Build workflow structure
	workflow := workflowDefinition{
		Name: "Dagu GitHub Action",
		On:   "push",
		Jobs: map[string]jobDefinition{
			"run": {
				RunsOn: "ubuntu-latest",
				Steps: []stepDefinition{
					{
						Uses: action,
						With: withParams,
					},
				},
			},
		},
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&workflow)
	if err != nil {
		return "", fmt.Errorf("failed to marshal workflow YAML: %w", err)
	}

	return string(yamlBytes), nil
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

	// Get runner image from step config, default to official Ubuntu latest
	runnerImage := "ubuntu:latest" // Official Ubuntu image (safe but limited tools)
	if img, ok := e.step.ExecutorConfig.Config["runner"]; ok {
		runnerImage = fmt.Sprintf("%v", img)
	}

	// Create act runner config with volume binding
	// workDir is the actual working directory where files will be checked out
	config := &runner.Config{
		Workdir:        workDir,
		BindWorkdir:    true,  // Bind the workdir to the container so files persist on host
		EventName:      "push",
		GitHubInstance: "github.com", // Configure GitHub instance for action resolution
		LogOutput:      true,          // Enable logging of docker run output (marked with raw_output field)
		Platforms: map[string]string{
			"ubuntu-latest": runnerImage,
		},
	}

	// Only set token if one is actually provided (for private repos/actions)
	if token != "" {
		config.Token = token
	}

	// Inject custom JobLoggerFactory into context
	// This routes container output (raw_output=true) to stdout and other logs to stderr
	loggerFactory := &daguJobLoggerFactory{
		stdout: e.stdout,
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
