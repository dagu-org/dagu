# Scheduler Health Check Design

## Overview

This design document outlines the implementation of an HTTP health check endpoint for the Dagu scheduler component. The health check will enable external monitoring systems (e.g., Kubernetes, monitoring tools) to verify that the scheduler is running and healthy.

## Requirements

1. **HTTP Health Endpoint**: Expose a `/health` endpoint that returns scheduler status
2. **Standalone Operation**: Only runs when scheduler is started via `dagu scheduler` command
3. **No Interference**: Does not run when using `dagu start-all` command
4. **Graceful Shutdown**: Properly handles system signals alongside the scheduler
5. **Configurable Port**: Default port 8090, configurable via config file
6. **Simple Response**: Returns JSON with status field

## Design

### Configuration

Add scheduler-specific configuration to the config structure:

```yaml
scheduler:
  port: 8090  # Default health check port
```

### Architecture

The health check server will be:
- A lightweight HTTP server running in a separate goroutine
- Started only when the scheduler command is executed directly
- Integrated with the scheduler's lifecycle (start/stop)
- Using the same signal handling pattern as the main scheduler

### Health Check Response

```json
{
  "status": "healthy"
}
```

The endpoint will return:
- **200 OK** with `{"status": "healthy"}` when scheduler is running
- **503 Service Unavailable** if scheduler is shutting down

### Implementation Components

#### 1. Configuration Changes

**File**: `internal/config/config.go`

Add scheduler configuration section:
```go
type SchedulerConfig struct {
    Port int `yaml:"port" mapstructure:"port" default:"8090"`
}

type Config struct {
    // ... existing fields ...
    Scheduler SchedulerConfig `yaml:"scheduler" mapstructure:"scheduler"`
}
```

#### 2. Health Server Implementation

**New File**: `internal/scheduler/health.go`

Create a dedicated health server that:
- Runs on the configured port
- Exposes `/health` endpoint
- Shares lifecycle with scheduler
- Uses Chi router for consistency

#### 3. Scheduler Integration

**File**: `internal/scheduler/scheduler.go`

Modify the scheduler to:
- Start health server in `Start()` method
- Stop health server in `Stop()` method
- Track health server lifecycle

#### 4. Command Integration

**File**: `internal/cmd/scheduler.go`

Ensure health server only starts when:
- Running `dagu scheduler` command
- NOT running via `dagu start-all`

### Signal Handling

The health server will:
1. Share the scheduler's context for cancellation
2. Shutdown gracefully when scheduler stops
3. Use the same shutdown timeout pattern

### Error Handling

1. **Port Already in Use**: Log error and continue scheduler operation
2. **Server Start Failure**: Log error but don't fail scheduler startup
3. **Graceful Shutdown**: Ensure clean shutdown even if health server fails

## Implementation Plan

### Phase 1: Configuration
1. Add scheduler config structure
2. Update config loader
3. Add default values

### Phase 2: Health Server
1. Create health server implementation
2. Add `/health` endpoint handler
3. Implement graceful shutdown

### Phase 3: Integration
1. Integrate with scheduler lifecycle
2. Update scheduler command
3. Ensure proper signal handling

### Phase 4: Testing
1. Unit tests for health server
2. Integration tests for scheduler with health check
3. Signal handling tests

## Security Considerations

1. **No Authentication**: Health endpoint is public (standard practice)
2. **Minimal Information**: Only return status, no sensitive data
3. **Local Only by Default**: Bind to localhost unless configured otherwise

## Monitoring Integration

The health endpoint enables:
1. **Kubernetes Probes**: Liveness and readiness checks
2. **Load Balancers**: Health verification
3. **Monitoring Tools**: Uptime tracking
4. **Alerting**: Detect scheduler failures

Example Kubernetes configuration:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8090
  initialDelaySeconds: 10
  periodSeconds: 30
```

## Future Considerations

While not in initial scope, future enhancements could include:
- Detailed health metrics (queue size, last run time)
- Readiness vs liveness distinction
- Metrics endpoint for Prometheus
- Health check for scheduler components (file watcher, queue reader)

## Testing Strategy

1. **Unit Tests**:
   - Health endpoint response
   - Server lifecycle
   - Configuration loading

2. **Integration Tests**:
   - Scheduler with health server
   - Signal handling
   - Port configuration

3. **Manual Testing**:
   - `dagu scheduler` starts health server
   - `dagu start-all` does not start health server
   - Graceful shutdown works correctly