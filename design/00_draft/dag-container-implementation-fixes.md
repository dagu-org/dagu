# DAG Container Implementation Fixes

## 1. Container Naming Fix

**Problem**: Container names need to be Docker-compliant and unique, but DAG names may contain invalid characters.

**Solution**: Use random IDs with consistent prefix from build package.

```go
// In constants.go
const (
    // Container placeholder used during DAG building
    ContainerNamePlaceholder = "DAGU_CONTAINER_PLACEHOLDER"
)

// In builder.go
func buildDAG(ctx BuildContext, def *dagDef, dag *DAG) error {
    // ... existing code ...
    
    if dag.Container != nil {
        // Use placeholder - will be replaced at runtime
        for _, step := range dag.Steps {
            transformStepForContainer(step, dag.Container, ContainerNamePlaceholder)
        }
    }
}

// In agent.go
import (
    "crypto/rand"
    "encoding/hex"
    "github.com/dagu-org/dagu/internal/build"
)

func (a *Agent) setupContainer(ctx context.Context) error {
    // Generate unique container name with app prefix
    a.containerName = fmt.Sprintf("%s-%s", build.Slug, generateRandomID())
    
    // Replace placeholder in all steps
    for _, step := range a.dag.Steps {
        if step.ExecutorConfig.Type == "docker" {
            if name, ok := step.ExecutorConfig.Config["containerName"].(string); ok && name == ContainerNamePlaceholder {
                step.ExecutorConfig.Config["containerName"] = a.containerName
            }
        }
    }
    
    // Continue with container creation...
}

func generateRandomID() string {
    b := make([]byte, 8)
    if _, err := rand.Read(b); err != nil {
        // Fallback to timestamp if random fails
        return fmt.Sprintf("%d", time.Now().UnixNano())
    }
    return hex.EncodeToString(b)
}
```

## 2. Missing Helper Functions

### Port Parsing
```go
func parsePortBindings(ports []string) nat.PortMap {
    portMap := make(nat.PortMap)
    
    for _, portSpec := range ports {
        // Parse "host:container[/protocol]"
        parts := strings.SplitN(portSpec, ":", 2)
        if len(parts) != 2 {
            logger.Warn(context.Background(), "Invalid port format", "port", portSpec)
            continue
        }
        
        hostPort := parts[0]
        containerSpec := parts[1]
        
        // Parse protocol (default tcp)
        protocol := "tcp"
        if protoParts := strings.SplitN(containerSpec, "/", 2); len(protoParts) == 2 {
            containerSpec = protoParts[0]
            protocol = protoParts[1]
        }
        
        containerPort := nat.Port(fmt.Sprintf("%s/%s", containerSpec, protocol))
        portMap[containerPort] = []nat.PortBinding{
            {
                HostIP:   "",
                HostPort: hostPort,
            },
        }
    }
    
    return portMap
}
```

### Build Context Creation
```go
func createBuildContext(contextPath, dockerfilePath string) (io.ReadCloser, error) {
    // Check context size to prevent huge uploads
    var totalSize int64
    maxSize := int64(100 * 1024 * 1024) // 100MB limit
    
    buf := new(bytes.Buffer)
    tw := tar.NewWriter(buf)
    
    // Read .dockerignore if exists
    ignorePatterns := readDockerignore(contextPath)
    
    err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        relPath, err := filepath.Rel(contextPath, path)
        if err != nil {
            return err
        }
        
        // Check ignore patterns
        if shouldIgnore(relPath, ignorePatterns) {
            if info.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }
        
        // Check size limit
        if !info.IsDir() {
            totalSize += info.Size()
            if totalSize > maxSize {
                return fmt.Errorf("build context too large: %d bytes (max %d)", totalSize, maxSize)
            }
        }
        
        // Create tar header
        header, err := tar.FileInfoHeader(info, path)
        if err != nil {
            return err
        }
        header.Name = relPath
        
        if err := tw.WriteHeader(header); err != nil {
            return err
        }
        
        // Write file content
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
        tw.Close()
        return nil, err
    }
    
    if err := tw.Close(); err != nil {
        return nil, err
    }
    
    return io.NopCloser(buf), nil
}

func readDockerignore(contextPath string) []string {
    ignorePath := filepath.Join(contextPath, ".dockerignore")
    file, err := os.Open(ignorePath)
    if err != nil {
        return nil
    }
    defer file.Close()
    
    var patterns []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line != "" && !strings.HasPrefix(line, "#") {
            patterns = append(patterns, line)
        }
    }
    return patterns
}
```

### Build Log Streaming
```go
func streamBuildLogs(ctx context.Context, reader io.Reader) error {
    type buildResponse struct {
        Stream      string `json:"stream"`
        Error       string `json:"error"`
        ErrorDetail struct {
            Message string `json:"message"`
        } `json:"errorDetail"`
    }
    
    decoder := json.NewDecoder(reader)
    for {
        var msg buildResponse
        if err := decoder.Decode(&msg); err != nil {
            if err == io.EOF {
                return nil
            }
            return fmt.Errorf("failed to decode build output: %w", err)
        }
        
        if msg.Error != "" {
            return fmt.Errorf("build failed: %s", msg.Error)
        }
        
        if msg.Stream != "" {
            // Log without trailing newline
            logger.Info(ctx, "Build", "output", strings.TrimRight(msg.Stream, "\n"))
        }
    }
}
```

## 3. Environment Variable Inheritance (Simplified)

**Set env only when creating container, not during step transformation:**

```go
// In transformStepForContainer - NO ENV
func transformStepForContainer(step *Step, container *Container, containerName string) {
    if step.ExecutorConfig.Type != "" {
        return
    }
    
    execConfig := map[string]any{
        "workingDir": container.WorkDir,
    }
    
    if container.User != "" {
        execConfig["user"] = container.User
    }
    
    // NO ENV HERE - container already has it
    
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

// In createAndStartContainer - SET ENV HERE
func (a *Agent) createAndStartContainer(ctx context.Context) error {
    // ... 
    
    containerConfig := &container.Config{
        Image:      a.dag.Container.Image,
        Env:        a.dag.Container.Env,  // Container gets all env vars
        WorkingDir: a.dag.Container.WorkDir,
        User:       a.dag.Container.User,
        Cmd:        []string{"/bin/sh", "-c", "while true; do sleep 30; done"},
    }
    
    // Docker exec will inherit the container's environment
}
```

## 4. Container Conflict Resolution

**Just remove existing container before creating new one:**

```go
func (a *Agent) setupContainer(ctx context.Context) error {
    a.containerName = fmt.Sprintf("dagu-%s-%s", a.dag.Name, a.dagRunID)
    
    // Remove any existing container with same name
    if err := a.removeExistingContainer(ctx); err != nil {
        logger.Warn(ctx, "Failed to remove existing container", "error", err)
        // Continue anyway - create might still work
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

func (a *Agent) removeExistingContainer(ctx context.Context) error {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return err
    }
    defer cli.Close()
    
    // Check if container exists
    _, err = cli.ContainerInspect(ctx, a.containerName)
    if err != nil {
        // Container doesn't exist, nothing to do
        return nil
    }
    
    logger.Info(ctx, "Removing existing container", "name", a.containerName)
    
    // Force remove (stops if running)
    removeOpts := container.RemoveOptions{
        Force: true,
        RemoveVolumes: true,
    }
    
    if err := cli.ContainerRemove(ctx, a.containerName, removeOpts); err != nil {
        return fmt.Errorf("failed to remove container: %w", err)
    }
    
    return nil
}
```

## Summary of Changes

1. **Container Naming**: Use existing special env vars (`DAG_NAME`, `DAG_RUN_ID`)
2. **Helper Functions**: Implemented with proper error handling and size limits
3. **Environment Variables**: Set once in container config, inherited by all exec operations
4. **Container Conflicts**: Force remove existing containers before creating new ones

These fixes address the identified gaps and make the implementation more robust and efficient.

## Note on Step-Level Overrides

We are NOT supporting step-level container overrides (`container: false`) in this implementation to keep things simple. All steps will run in the DAG container if one is specified. Users who need different execution environments should use separate DAGs or the existing executor configuration.