# DAG-Level Container Implementation

## Overview

When a DAG specifies a container, we:
1. Create a container at DAG startup
2. Transform all steps to use Docker executor in exec mode
3. Remove container on DAG completion

## Implementation Approach

### DAG Container Lifecycle

**Container Creation (at DAG start):**
- Build image if `build` specified
- Create container with name: `dagu-${DAG_NAME}-${DAG_RUN_ID}`
- Start container with long-running command (e.g., `/bin/sh -c "while true; do sleep 30; done"`)

**Container Removal (at DAG end):**
- Stop and remove container (unless `keepContainer: true`)

### Step Transformation

When DAG has container configuration, transform each step during DAG building:

```yaml
# Original step:
steps:
  - name: test
    command: npm test

# Transformed to:
steps:
  - name: test
    executor:
      type: docker
      config:
        containerName: dagu-${DAG_NAME}-${DAG_RUN_ID}
        exec:
          workingDir: /workspace  # from container config
          user: "1000"            # from container config
          env:                    # merged from container + DAG
            - FOO=bar
    command: npm test
```

## Core Changes

### DAG Definition
**File: `internal/digraph/definition.go`**

```go
type containerDef struct {
    Image         string            `yaml:"image,omitempty"`
    Build         *buildDef         `yaml:"build,omitempty"`
    Env           []string          `yaml:"env,omitempty"`
    Volumes       []string          `yaml:"volumes,omitempty"`
    User          string            `yaml:"user,omitempty"`
    WorkDir       string            `yaml:"workDir,omitempty"`
    KeepContainer bool              `yaml:"keepContainer,omitempty"`
}
```

### DAG Builder
**File: `internal/digraph/builder.go`**

In `buildDAG` function, only transform steps:

```go
func buildDAG(ctx BuildContext, def *dagDef, dag *DAG) error {
    // ... existing code ...
    
    if def.Container != nil {
        // Store container config in DAG
        dag.Container = buildContainer(def.Container)
        
        // Transform all steps to use docker exec
        // Use placeholder - will be replaced at runtime
        for _, step := range dag.Steps {
            transformStepToDockerExec(step, dag.Container, ContainerNamePlaceholder)
        }
    }
}
```

### Agent Container Management
**File: `internal/agent/agent.go`**

The Agent handles container lifecycle directly:

```go
func (a *Agent) Run(ctx context.Context) error {
    // ... existing setup ...
    
    if a.dag.Container != nil {
        if err := a.setupContainer(ctx); err != nil {
            return fmt.Errorf("failed to setup container: %w", err)
        }
        
        // Ensure cleanup
        defer func() {
            if !a.dag.Container.KeepContainer {
                a.cleanupContainer(context.Background())
            }
        }()
    }
    
    // Continue with normal execution
    return a.scheduler.Schedule(ctx)
}

func (a *Agent) setupContainer(ctx context.Context) error {
    // Generate unique container name
    a.containerName = fmt.Sprintf("%s-%s", build.Slug, generateRandomID())
    
    // Replace placeholder in all steps
    for _, step := range a.dag.Steps {
        if step.ExecutorConfig.Type == "docker" {
            if name, ok := step.ExecutorConfig.Config["containerName"].(string); ok && name == ContainerNamePlaceholder {
                step.ExecutorConfig.Config["containerName"] = a.containerName
            }
        }
    }
    
    // Remove any existing container
    if err := a.removeExistingContainer(ctx); err != nil {
        logger.Warn(ctx, "Failed to remove existing container", "error", err)
    }
    
    // Build image if needed
    if a.dag.Container.Build != nil {
        if err := a.buildContainerImage(ctx); err != nil {
            return err
        }
    }
    
    // Create and start container
    return a.createAndStartContainer(ctx)
}
```

### Step Transformation Logic

```go
func transformStepToDockerExec(step *Step, container *containerDef, containerName string) {
    // Skip if step already has executor config
    if step.ExecutorConfig.Type != "" {
        return
    }
    
    step.ExecutorConfig = ExecutorConfig{
        Type: "docker",
        Config: map[string]any{
            "containerName": containerName,
            "exec": map[string]any{
                "workingDir": container.WorkDir,
                "user": container.User,
                "env": mergeEnv(container.Env),
            },
        },
    }
}
```


### Build Support

If `build` is specified instead of `image`, the Agent handles the build:

```go
func (a *Agent) createContainer(ctx context.Context, name string) error {
    // If build config exists, build image first
    if a.dag.Container.Build != nil {
        imageName := fmt.Sprintf("dagu-build-%s-%s", a.dag.Name, a.dagRunID)
        if err := a.buildImage(ctx, imageName); err != nil {
            return fmt.Errorf("failed to build image: %w", err)
        }
        // Use built image
        a.dag.Container.Image = imageName
    }
    
    // Continue with container creation using the image
}
```

## Environment Variable Handling

Merge in this order (BuildContext already handles DAG env):
1. DAG env (already in context)
2. Container env (added during transformation)

## Benefits

- Reuses existing Docker executor
- No new execution paths
- Leverages existing error handling
- Compatible with existing features (retry, output capture, etc.)
- Minimal code changes

## Edge Cases

- Step with explicit executor: skip transformation
- Step with `container: false`: run on host (skip transformation)
- Container creation failure: fail fast
- Cleanup always runs via `handlerOn.exit`