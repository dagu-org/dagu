package gha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/docker/docker/api/types/container"
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
    command: actions/checkout@v4
    executor:
      type: gha
    params:
      repository: myorg/myrepo
      ref: main

  - name: setup-go
    command: actions/setup-go@v5
    executor:
      type: gha
    params:
      go-version: '1.23'
```
*/

const (
	// Config keys for GitHub Action executor
	configKeyRunner          = "runner"          // The Docker image to use as runner
	configKeyAutoRemove      = "autoRemove"      // Automatically remove containers after execution
	configKeyNetwork         = "network"         // Docker network mode
	configKeyGitHubInstance  = "githubInstance"  // GitHub instance for action resolution
	configKeyDockerSocket    = "dockerSocket"    // Custom Docker socket path
	configKeyArtifacts       = "artifacts"       // Artifact server configuration
	configKeyReuseContainers = "reuseContainers" // Reuse containers between runs
	configKeyForceRebuild    = "forceRebuild"    // Force rebuild of action images
	configKeyContainerOpts   = "containerOptions" // Additional Docker run options
	configKeyPrivileged      = "privileged"      // Run containers in privileged mode
	configKeyCapabilities    = "capabilities"    // Linux capabilities configuration

	// Act configuration defaults
	defaultRunnerImage  = "catthehacker/ubuntu:act-latest" // Medium-size runner with common tools
	defaultPlatform     = "ubuntu-latest"
	defaultEventName    = "push"
	defaultGitHubHost   = "github.com"
	defaultWorkflowName = "Dagu GitHub Action"
)

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
	// All other logs go to stderr - write timestamp + tab + message
	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	msg := entry.Message
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	_, err := h.stderr.Write([]byte(timestamp + "\t" + msg))
	return err
}

// WithJobLogger creates a logrus logger that routes output appropriately
func (f *daguJobLoggerFactory) WithJobLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Disable default output, use hook instead
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:    false,
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05",
		DisableTimestamp: false,
	})
	// Add hook to route output: raw_output to stdout, everything else to stderr
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
	actualWorkDir := execution.GetEnv(ctx).WorkingDir
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
	workflowYAML, err := e.generateWorkflowYAML(ctx)
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
	return e.executeAct(ctx, actualWorkDir, tmpDir, workflowFile)
}

// workflowDefinition represents a GitHub Actions workflow
type workflowDefinition struct {
	Name string                   `yaml:"name"`
	On   string                   `yaml:"on"`
	Env  map[string]string        `yaml:"env,omitempty"`
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

func (e *githubAction) generateWorkflowYAML(ctx context.Context) (string, error) {
	if e.step.Command == "" {
		return "", fmt.Errorf("command field is required for GitHub Action executor (e.g., command: actions/checkout@v4)")
	}

	action := strings.TrimSpace(e.step.Command)
	env := execution.GetEnv(ctx)

	// Get action inputs from step.Params
	paramsMap, err := e.step.Params.AsStringMap()
	if err != nil {
		return "", fmt.Errorf("failed to get params as map: %w", err)
	}

	// Evaluate and copy action inputs to avoid mutation
	withParams := make(map[string]any, len(paramsMap))
	for k, v := range paramsMap {
		val, err := env.EvalString(ctx, v)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate action input %q: %w", k, err)
		}
		withParams[k] = val
	}

	// Build workflow-level environment variables from execution context
	// This includes DAG env, Variables from previous steps, step env
	// Secrets are excluded (they're passed separately via act's Config.Secrets)
	workflowEnv := env.UserEnvsMap()
	for k := range env.SecretEnvs {
		delete(workflowEnv, k)
	}

	// Build workflow structure
	workflow := workflowDefinition{
		Name: defaultWorkflowName,
		On:   defaultEventName,
		Env:  workflowEnv,
		Jobs: map[string]jobDefinition{
			"run": {
				RunsOn: defaultPlatform,
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

// generateEventJSON creates a GitHub webhook event payload for the action
// This provides the github.event context to actions
func (e *githubAction) generateEventJSON() (string, error) {
	// Create a minimal push event payload
	// This provides basic GitHub context for actions
	event := map[string]any{
		"ref": "refs/heads/main",
		"repository": map[string]any{
			"name":      "dagu",
			"full_name": "dagu-org/dagu",
			"private":   false,
		},
		"pusher": map[string]any{
			"name": "dagu",
		},
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event.json: %w", err)
	}

	return string(eventBytes), nil
}

func (e *githubAction) executeAct(ctx context.Context, workDir, tmpDir, workflowFile string) error {
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

	// Get execution environment to access secrets and env vars
	env := execution.GetEnv(ctx)

	// Get all user-defined environment variables (excludes OS env, includes Variables)
	// Includes: DAG env + step env + variables from previous steps
	actEnv := env.UserEnvsMap()

	// Build secrets map for act and remove secrets from actEnv
	// Secrets should only be passed via Config.Secrets, not Config.Env
	actSecrets := make(map[string]string)
	for k, v := range env.SecretEnvs {
		actSecrets[k] = v
		delete(actEnv, k) // Remove from env to avoid duplication
	}

	// Generate event.json for GitHub context
	eventJSON, err := e.generateEventJSON()
	if err != nil {
		return fmt.Errorf("failed to generate event.json: %w", err)
	}

	// Write event.json to temp directory
	// Use step name to make it unique in case of concurrent executions
	eventFile := filepath.Join(tmpDir, fmt.Sprintf("event-%s.json", e.step.Name))
	if err := os.WriteFile(eventFile, []byte(eventJSON), 0644); err != nil {
		return fmt.Errorf("failed to write event.json: %w", err)
	}

	// Build runner config from step executor config
	config := e.buildRunnerConfig(workDir, eventFile, actEnv, actSecrets)

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
	plan, err := planner.PlanEvent(defaultEventName)
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

// buildRunnerConfig constructs runner.Config from step executor config
func (e *githubAction) buildRunnerConfig(workDir, eventFile string, actEnv, actSecrets map[string]string) *runner.Config {
	cfg := e.step.ExecutorConfig.Config

	// Get runner image, default to official Node.js image
	runnerImage := defaultRunnerImage
	if img, ok := cfg[configKeyRunner]; ok {
		runnerImage = fmt.Sprintf("%v", img)
	}

	// Get GitHub instance, default to github.com
	githubInstance := defaultGitHubHost
	if instance, ok := cfg[configKeyGitHubInstance]; ok {
		githubInstance = fmt.Sprintf("%v", instance)
	}

	// Get autoRemove, default to true for security and disk space
	autoRemove := true
	if val, ok := cfg[configKeyAutoRemove]; ok {
		if b, ok := val.(bool); ok {
			autoRemove = b
		}
	}

	// Get reuseContainers, default to false
	reuseContainers := false
	if val, ok := cfg[configKeyReuseContainers]; ok {
		if b, ok := val.(bool); ok {
			reuseContainers = b
		}
	}

	// Get forceRebuild, default to false
	forceRebuild := false
	if val, ok := cfg[configKeyForceRebuild]; ok {
		if b, ok := val.(bool); ok {
			forceRebuild = b
		}
	}

	// Get privileged mode, default to false (SECURITY: should never default to true)
	privileged := false
	if val, ok := cfg[configKeyPrivileged]; ok {
		if b, ok := val.(bool); ok {
			privileged = b
		}
	}

	// Get network mode, default to empty (Docker default)
	networkMode := ""
	if val, ok := cfg[configKeyNetwork]; ok {
		networkMode = fmt.Sprintf("%v", val)
	}

	// Get Docker socket, default to empty (Docker default)
	dockerSocket := ""
	if val, ok := cfg[configKeyDockerSocket]; ok {
		dockerSocket = fmt.Sprintf("%v", val)
	}

	// Get container options, default to empty
	containerOpts := ""
	if val, ok := cfg[configKeyContainerOpts]; ok {
		containerOpts = fmt.Sprintf("%v", val)
	}

	// Parse capabilities configuration
	var capAdd, capDrop []string
	if capsVal, ok := cfg[configKeyCapabilities]; ok {
		if capsMap, ok := capsVal.(map[string]any); ok {
			if add, ok := capsMap["add"]; ok {
				capAdd = parseStringSlice(add)
			}
			if drop, ok := capsMap["drop"]; ok {
				capDrop = parseStringSlice(drop)
			}
		}
	}

	// Parse artifacts configuration
	var artifactServerPath, artifactServerPort string
	if artifactsVal, ok := cfg[configKeyArtifacts]; ok {
		if artifactsMap, ok := artifactsVal.(map[string]any); ok {
			if path, ok := artifactsMap["path"]; ok {
				artifactServerPath = fmt.Sprintf("%v", path)
			}
			if port, ok := artifactsMap["port"]; ok {
				artifactServerPort = fmt.Sprintf("%v", port)
			}
		}
	}

	// Build runner config
	config := &runner.Config{
		// Core configuration
		Workdir:        workDir,
		BindWorkdir:    true,
		EventName:      defaultEventName,
		EventPath:      eventFile,
		GitHubInstance: githubInstance,
		LogOutput:      true,
		Env:            actEnv,
		Secrets:        actSecrets,
		Platforms: map[string]string{
			defaultPlatform: runnerImage,
		},

		// Container lifecycle
		AutoRemove:      autoRemove,
		ReuseContainers: reuseContainers,
		ForceRebuild:    forceRebuild,

		// Docker configuration
		ContainerDaemonSocket: dockerSocket,
		ContainerOptions:      containerOpts,

		// Security
		Privileged:       privileged,
		ContainerCapAdd:  capAdd,
		ContainerCapDrop: capDrop,

		// Artifacts
		ArtifactServerPath: artifactServerPath,
		ArtifactServerPort: artifactServerPort,
	}

	// Set network mode if specified (requires type conversion)
	if networkMode != "" {
		config.ContainerNetworkMode = container.NetworkMode(networkMode)
	}

	return config
}

// parseStringSlice converts various input types to []string
func parseStringSlice(input any) []string {
	switch v := input.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case string:
		// Single string, return as single-element slice
		if v != "" {
			return []string{v}
		}
		return nil
	default:
		return nil
	}
}

func newGitHubAction(ctx context.Context, step core.Step) (executor.Executor, error) {
	if step.Command == "" {
		return nil, fmt.Errorf("command field is required for GitHub Action executor (e.g., command: actions/checkout@v4)")
	}

	return &githubAction{
		step:   step,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}, nil
}

func init() {
	executor.RegisterExecutor("github_action", newGitHubAction, nil)
	executor.RegisterExecutor("github-action", newGitHubAction, nil)
	executor.RegisterExecutor("gha", newGitHubAction, nil)
}
