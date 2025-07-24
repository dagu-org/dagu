# DAG Container Detailed Implementation

## 1. Definition Layer

### 1.1 YAML Definition Structure

**File: `internal/digraph/definition.go`**

```go
// Add to existing file
type dagDef struct {
    // ... existing fields ...
    Container *containerDef `yaml:"container,omitempty"`
}

type containerDef struct {
    Image         string            `yaml:"image,omitempty"`
    Build         *buildDef         `yaml:"build,omitempty"`
    Env           []string          `yaml:"env,omitempty"`
    Volumes       []string          `yaml:"volumes,omitempty"`
    User          string            `yaml:"user,omitempty"`
    WorkDir       string            `yaml:"workDir,omitempty"`
    Pull          string            `yaml:"pull,omitempty"`
    Platform      string            `yaml:"platform,omitempty"`
    Ports         []string          `yaml:"ports,omitempty"`
    Network       string            `yaml:"network,omitempty"`
    KeepContainer bool              `yaml:"keepContainer,omitempty"`
}

type buildDef struct {
    Dockerfile string            `yaml:"dockerfile,omitempty"`
    Context    string            `yaml:"context,omitempty"`
    Args       map[string]string `yaml:"args,omitempty"`
}
```

### 1.2 DAG Model Structure

**File: `internal/digraph/dag.go`**

```go
// Add to DAG struct
type DAG struct {
    // ... existing fields ...
    Container *Container `json:"container,omitempty"`
}

// New types
type Container struct {
    Image         string
    Build         *BuildConfig
    Env           []string
    Volumes       []string
    User          string
    WorkDir       string
    Pull          string
    Platform      string
    Ports         []string
    Network       string
    KeepContainer bool
}

type BuildConfig struct {
    Dockerfile string
    Context    string
    Args       map[string]string
}
```

## 2. Builder Layer

### 2.1 Container Builder

**File: `internal/digraph/builder.go`**

```go
// Add to buildDAG function after existing build logic
func buildDAG(ctx BuildContext, def *dagDef, dag *DAG) error {
    // ... existing code ...
    
    // Build container config if present
    if def.Container != nil {
        dag.Container = buildContainer(def.Container)
    }
    
    // ... continue with step building ...
    
    // After steps are built, transform them if container exists
    if dag.Container != nil {
        containerName := fmt.Sprintf("dagu-%s-${DAG_RUN_ID}", dag.Name)
        for _, step := range dag.Steps {
            transformStepForContainer(step, dag.Container, containerName)
        }
    }
    
    return nil
}

func buildContainer(def *containerDef) *Container {
    c := &Container{
        Image:         def.Image,
        Env:           def.Env,
        Volumes:       def.Volumes,
        User:          def.User,
        WorkDir:       def.WorkDir,
        Pull:          def.Pull,
        Platform:      def.Platform,
        Ports:         def.Ports,
        Network:       def.Network,
        KeepContainer: def.KeepContainer,
    }
    
    // Set defaults
    if c.WorkDir == "" {
        c.WorkDir = "/workspace"
    }
    if c.Pull == "" {
        c.Pull = "missing"
    }
    if len(c.Volumes) == 0 {
        c.Volumes = []string{".:/workspace"}
    }
    
    // Build config if present
    if def.Build != nil {
        c.Build = &BuildConfig{
            Dockerfile: def.Build.Dockerfile,
            Context:    def.Build.Context,
            Args:       def.Build.Args,
        }
        // Set build defaults
        if c.Build.Dockerfile == "" {
            c.Build.Dockerfile = "./Dockerfile"
        }
        if c.Build.Context == "" {
            c.Build.Context = "."
        }
    }
    
    return c
}
```

### 2.2 Step Transformation

**File: `internal/digraph/builder.go`**

```go
func transformStepForContainer(step *Step, container *Container, containerName string) {
    // Skip if step already has executor
    if step.ExecutorConfig.Type != "" {
        return
    }
    
    // Skip if step explicitly disables container
    // TODO: Add container: false support to stepDef
    
    // Create docker exec configuration
    execConfig := map[string]any{
        "workingDir": container.WorkDir,
    }
    
    if container.User != "" {
        execConfig["user"] = container.User
    }
    
    if len(container.Env) > 0 {
        execConfig["env"] = container.Env
    }
    
    // Override with step's dir if specified
    if step.Dir != "" {
        execConfig["workingDir"] = step.Dir
    }
    
    step.ExecutorConfig = ExecutorConfig{
        Type: "docker",
        Config: map[string]any{
            "containerName": containerName,
            "exec": execConfig,
        },
    }
}
```

## 3. Agent Layer

### 3.1 Agent Container Management

**File: `internal/agent/agent.go`**

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    // ... other imports
)

// Add containerName field to Agent
type Agent struct {
    // ... existing fields ...
    containerName string
}

// Modify Run method
func (a *Agent) Run(ctx context.Context) error {
    setup := func() error {
        a.init()
        
        // Create container if DAG has container config
        if a.dag.Container != nil {
            if err := a.setupContainer(ctx); err != nil {
                return fmt.Errorf("failed to setup container: %w", err)
            }
        }
        
        return a.setupInternal(ctx)
    }
    
    if err := setup(); err != nil {
        return err
    }
    
    // Ensure cleanup
    defer func() {
        if a.dag.Container != nil && !a.dag.Container.KeepContainer {
            if err := a.cleanupContainer(context.Background()); err != nil {
                logger.Error(ctx, "Failed to cleanup container", "error", err)
            }
        }
    }()
    
    return a.scheduler.Schedule(ctx)
}
```

### 3.2 Container Lifecycle Methods

**File: `internal/agent/agent.go`**

```go
func (a *Agent) setupContainer(ctx context.Context) error {
    a.containerName = fmt.Sprintf("dagu-%s-%s", a.dag.Name, a.dagRunID)
    
    // Build image if needed
    if a.dag.Container.Build != nil {
        if err := a.buildContainerImage(ctx); err != nil {
            return err
        }
    }
    
    // Create and start container
    return a.createAndStartContainer(ctx)
}

func (a *Agent) buildContainerImage(ctx context.Context) error {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return err
    }
    defer cli.Close()
    
    // Generate image name
    imageName := fmt.Sprintf("dagu-build-%s:%s", a.dag.Name, a.dagRunID)
    
    // Build context
    buildCtx, err := createBuildContext(a.dag.Container.Build.Context, a.dag.Container.Build.Dockerfile)
    if err != nil {
        return fmt.Errorf("failed to create build context: %w", err)
    }
    defer buildCtx.Close()
    
    // Build options
    buildOptions := types.ImageBuildOptions{
        Dockerfile: a.dag.Container.Build.Dockerfile,
        Tags:       []string{imageName},
        BuildArgs:  a.dag.Container.Build.Args,
        Remove:     true,
        PullParent: a.dag.Container.Pull == "always",
    }
    
    // Build image
    resp, err := cli.ImageBuild(ctx, buildCtx, buildOptions)
    if err != nil {
        return fmt.Errorf("failed to build image: %w", err)
    }
    defer resp.Body.Close()
    
    // Stream build output to logger
    if err := streamBuildLogs(ctx, resp.Body); err != nil {
        return err
    }
    
    // Update container image to use built image
    a.dag.Container.Image = imageName
    
    return nil
}

func (a *Agent) createAndStartContainer(ctx context.Context) error {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return err
    }
    defer cli.Close()
    
    // Container config
    containerConfig := &container.Config{
        Image:      a.dag.Container.Image,
        Env:        a.dag.Container.Env,
        WorkingDir: a.dag.Container.WorkDir,
        User:       a.dag.Container.User,
        // Keep container running
        Cmd: []string{"/bin/sh", "-c", "while true; do sleep 30; done"},
    }
    
    // Host config
    hostConfig := &container.HostConfig{
        Binds:      a.dag.Container.Volumes,
        AutoRemove: false, // We'll remove manually
    }
    
    // Parse and set port bindings
    if len(a.dag.Container.Ports) > 0 {
        hostConfig.PortBindings = parsePortBindings(a.dag.Container.Ports)
    }
    
    // Network config
    networkConfig := &network.NetworkingConfig{}
    if a.dag.Container.Network != "" {
        // TODO: Configure network mode
    }
    
    // Platform
    var platform *specs.Platform
    if a.dag.Container.Platform != "" {
        p := platforms.MustParse(a.dag.Container.Platform)
        platform = &p
    }
    
    // Create container
    resp, err := cli.ContainerCreate(
        ctx,
        containerConfig,
        hostConfig,
        networkConfig,
        platform,
        a.containerName,
    )
    if err != nil {
        return fmt.Errorf("failed to create container: %w", err)
    }
    
    // Start container
    if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
        return fmt.Errorf("failed to start container: %w", err)
    }
    
    logger.Info(ctx, "Container started", "name", a.containerName, "id", resp.ID)
    
    return nil
}

func (a *Agent) cleanupContainer(ctx context.Context) error {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return err
    }
    defer cli.Close()
    
    // Stop container
    timeout := 10 // seconds
    if err := cli.ContainerStop(ctx, a.containerName, container.StopOptions{
        Timeout: &timeout,
    }); err != nil {
        logger.Warn(ctx, "Failed to stop container", "error", err)
    }
    
    // Remove container
    if err := cli.ContainerRemove(ctx, a.containerName, container.RemoveOptions{
        Force: true,
    }); err != nil {
        return fmt.Errorf("failed to remove container: %w", err)
    }
    
    logger.Info(ctx, "Container removed", "name", a.containerName)
    
    return nil
}
```

### 3.3 Build Context Helper

**File: `internal/agent/build.go`**

```go
func createBuildContext(contextPath, dockerfilePath string) (io.ReadCloser, error) {
    // Create tar archive of build context
    buf := new(bytes.Buffer)
    tw := tar.NewWriter(buf)
    defer tw.Close()
    
    // Walk directory and add files to tar
    err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        // Skip .dockerignore files
        if matched, _ := filepath.Match(".dockerignore", filepath.Base(path)); matched {
            return nil
        }
        
        // Create tar header
        header, err := tar.FileInfoHeader(info, path)
        if err != nil {
            return err
        }
        
        // Update header name to be relative to context
        relPath, err := filepath.Rel(contextPath, path)
        if err != nil {
            return err
        }
        header.Name = relPath
        
        // Write header
        if err := tw.WriteHeader(header); err != nil {
            return err
        }
        
        // Write file content if not directory
        if !info.IsDir() {
            file, err := os.Open(path)
            if err != nil {
                return err
            }
            defer file.Close()
            
            if _, err := io.Copy(tw, file); err != nil {
                return err
            }
        }
        
        return nil
    })
    
    if err != nil {
        return nil, err
    }
    
    return io.NopCloser(buf), nil
}

func streamBuildLogs(ctx context.Context, reader io.Reader) error {
    decoder := json.NewDecoder(reader)
    for {
        var msg map[string]interface{}
        if err := decoder.Decode(&msg); err != nil {
            if err == io.EOF {
                break
            }
            return err
        }
        
        // Log build output
        if stream, ok := msg["stream"].(string); ok {
            logger.Info(ctx, "Build", "output", strings.TrimSpace(stream))
        }
        
        // Check for errors
        if errMsg, ok := msg["error"].(string); ok {
            return fmt.Errorf("build error: %s", errMsg)
        }
    }
    return nil
}
```

## 4. API Layer

### 4.1 OpenAPI Schema Updates

**File: `api/v2/api.yaml`**

Add to DAGDetails schema:
```yaml
DAGDetails:
  type: object
  properties:
    # ... existing fields ...
    container:
      $ref: "#/components/schemas/Container"

Container:
  type: object
  description: "Container configuration for running all DAG steps"
  properties:
    image:
      type: string
      description: "Docker image to use"
    build:
      $ref: "#/components/schemas/ContainerBuild"
    env:
      type: array
      items:
        type: string
      description: "Environment variables"
    volumes:
      type: array
      items:
        type: string
      description: "Volume mounts"
    user:
      type: string
      description: "User to run as"
    workDir:
      type: string
      description: "Working directory"
    keepContainer:
      type: boolean
      description: "Keep container after DAG completion"

ContainerBuild:
  type: object
  description: "Build configuration for container"
  properties:
    dockerfile:
      type: string
      description: "Path to Dockerfile"
    context:
      type: string
      description: "Build context path"
    args:
      type: object
      additionalProperties:
        type: string
      description: "Build arguments"
```

### 4.2 API Transformer Updates

**File: `internal/frontend/api/v2/transformer/dag.go`**

```go
func toAPIDAGDetailsResponse(d *digraph.DAGDetails) *api.DAGDetails {
    // ... existing code ...
    
    if d.DAG.Container != nil {
        resp.Container = toAPIContainer(d.DAG.Container)
    }
    
    return resp
}

func toAPIContainer(c *digraph.Container) *api.Container {
    ac := &api.Container{
        Image:         &c.Image,
        Env:           &c.Env,
        Volumes:       &c.Volumes,
        User:          &c.User,
        WorkDir:       &c.WorkDir,
        KeepContainer: &c.KeepContainer,
    }
    
    if c.Build != nil {
        ac.Build = &api.ContainerBuild{
            Dockerfile: &c.Build.Dockerfile,
            Context:    &c.Build.Context,
            Args:       &c.Build.Args,
        }
    }
    
    return ac
}
```

## 5. Testing

### 5.1 Unit Tests

**File: `internal/digraph/builder_test.go`**

```go
func TestContainerTransformation(t *testing.T) {
    tests := []struct {
        name     string
        yaml     string
        expected int // number of steps with docker executor
    }{
        {
            name: "dag with container",
            yaml: `
name: test
container:
  image: node:18
steps:
  - name: step1
    command: echo hello
  - name: step2
    command: echo world
`,
            expected: 2,
        },
        {
            name: "dag with build",
            yaml: `
name: test
container:
  build:
    dockerfile: ./Dockerfile
    context: .
steps:
  - name: build
    command: npm build
`,
            expected: 1,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dag, err := Load([]byte(tt.yaml))
            require.NoError(t, err)
            
            // Count steps with docker executor
            count := 0
            for _, step := range dag.Steps {
                if step.ExecutorConfig.Type == "docker" {
                    count++
                }
            }
            
            assert.Equal(t, tt.expected, count)
        })
    }
}
```

### 5.2 Integration Tests

**File: `internal/agent/agent_test.go`**

```go
func TestContainerLifecycle(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Create DAG with container
    dag := &digraph.DAG{
        Name: "test-container",
        Container: &digraph.Container{
            Image: "alpine:latest",
            Env: []string{"TEST=true"},
        },
        Steps: []*digraph.Step{
            {
                Name:    "test",
                Command: "echo",
                Args:    []string{"$TEST"},
            },
        },
    }
    
    // Run agent
    agent := New(dag, "", nil)
    err := agent.Run(context.Background())
    
    require.NoError(t, err)
    
    // Verify container was cleaned up
    // TODO: Check container doesn't exist
}
```