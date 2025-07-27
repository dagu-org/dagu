# etcd Lock Mechanism Requirements for Dagu HA Mode

## Document Status
- **Type**: Feature Requirements
- **Date**: 2025-01-27
- **Status**: Draft
- **Related**: [ha_mode_requirements.md](./ha_mode_requirements.md)

## Executive Summary

This document proposes using etcd purely as a distributed lock mechanism to improve Dagu's HA mode. By replacing the file-based dirlock with etcd locks, we gain faster failover, better reliability, and eliminate stale lock issues - all with minimal code changes.

## Design Philosophy

**Keep It Simple**: Use etcd only for what it does best - distributed locks. Continue using the file-based storage for everything else (DAGs, logs, state, queue).

## Why etcd for Locks?

### Current dirlock Limitations
1. **Stale Lock Detection**: Relies on timestamp checks (15-30 second delays)
2. **No Active Health Monitoring**: Can't detect hung processes quickly
3. **Manual Refresh Required**: Complex heartbeat implementation needed
4. **Split-Brain Risk**: Brief periods where multiple schedulers might run

### etcd Lock Advantages
1. **Session-Based TTL**: Automatic expiration when process dies
2. **Fast Detection**: 3-5 second failover vs 15-30 seconds
3. **Automatic Keepalive**: No manual refresh needed
4. **Strong Consistency**: Raft consensus prevents split-brain

## Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Dagu #1    │     │  Dagu #2    │     │  Dagu #3    │
│  (Leader)   │     │ (Standby)   │     │ (Standby)   │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │ Holds Lock        │ Waits for Lock  │ Waits for Lock
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                    ┌──────▼──────┐
                    │    etcd     │
                    │   (Locks    │
                    │    Only)    │
                    └─────────────┘
                           │
                    Shared Filesystem
                    (Everything Else:
                     DAGs, Logs, State,
                     Queue, Data)
```

## Configuration Design

### Simple Configuration Structure

```go
// internal/config/config.go

type Config struct {
    // ... existing fields ...
    
    // HA mode with etcd locks
    HA *HAConfig `mapstructure:"ha"`
}

type HAConfig struct {
    // Enable HA mode with etcd locks
    Enabled bool `mapstructure:"enabled"`
    
    // etcd configuration for locks only
    Etcd *EtcdLockConfig `mapstructure:"etcd"`
}

type EtcdLockConfig struct {
    // etcd endpoints
    Endpoints []string `mapstructure:"endpoints"`
    
    // Lock namespace prefix
    LockPrefix string `mapstructure:"lockPrefix"`
    
    // Lock TTL (how long before lock expires)
    LockTTL time.Duration `mapstructure:"lockTTL"`
    
    // Connection timeout
    DialTimeout time.Duration `mapstructure:"dialTimeout"`
    
    // Authentication
    Username string `mapstructure:"username"`
    Password string `mapstructure:"password"`
    
    // TLS configuration
    TLS *TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    CertFile string `mapstructure:"certFile"`
    KeyFile  string `mapstructure:"keyFile"`
    CAFile   string `mapstructure:"caFile"`
}
```

### Configuration Example

```yaml
# Enable HA mode with etcd locks
ha:
  enabled: true
  
  etcd:
    # etcd cluster endpoints
    endpoints:
      - "https://etcd1.example.com:2379"
      - "https://etcd2.example.com:2379"
      - "https://etcd3.example.com:2379"
    
    # Lock namespace (allows multiple Dagu clusters)
    lockPrefix: "/dagu/prod/locks"
    
    # Lock expires after 15 seconds if holder dies
    lockTTL: 15s
    
    # Connection timeout
    dialTimeout: 5s
    
    # Optional authentication
    username: ${ETCD_USERNAME}
    password: ${ETCD_PASSWORD}
    
    # TLS for production
    tls:
      enabled: true
      certFile: /etc/dagu/certs/etcd-client.crt
      keyFile: /etc/dagu/certs/etcd-client.key
      caFile: /etc/dagu/certs/etcd-ca.crt

# Everything else remains the same
# File-based storage for DAGs, logs, state, queue
dataDir: /var/dagu/data
logDir: /var/dagu/logs
```

### Environment Variables

```bash
# etcd connection
DAGU_HA_ENABLED=true
DAGU_HA_ETCD_ENDPOINTS="https://etcd1:2379,https://etcd2:2379"
DAGU_HA_ETCD_LOCKPREFIX="/dagu/prod/locks"
DAGU_HA_ETCD_LOCKTTL="15s"

# Authentication
DAGU_HA_ETCD_USERNAME="dagu-service"
DAGU_HA_ETCD_PASSWORD="secure-password"

# TLS
DAGU_HA_ETCD_TLS_ENABLED=true
DAGU_HA_ETCD_TLS_CERTFILE="/certs/client.crt"
DAGU_HA_ETCD_TLS_KEYFILE="/certs/client.key"
DAGU_HA_ETCD_TLS_CAFILE="/certs/ca.crt"
```

## Lock Implementation Design

### Lock Interface

```go
// internal/persistence/lock/lock.go
type Lock interface {
    TryLock() error
    Unlock() error
    IsLocked() bool
}

// Factory function based on config
func NewLock(path string, config *config.Config) (Lock, error) {
    if config.HA != nil && config.HA.Enabled && config.HA.Etcd != nil {
        return newEtcdLock(path, config.HA.Etcd)
    }
    // Fall back to dirlock
    return dirlock.New(path), nil
}
```

### etcd Lock Implementation

```go
// internal/persistence/etcdlock/etcdlock.go
type EtcdLock struct {
    client  *clientv3.Client
    session *concurrency.Session
    mutex   *concurrency.Mutex
    locked  bool
}

func newEtcdLock(path string, config *config.EtcdLockConfig) (*EtcdLock, error) {
    // Create etcd client
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   config.Endpoints,
        DialTimeout: config.DialTimeout,
        Username:    config.Username,
        Password:    config.Password,
        TLS:         buildTLSConfig(config.TLS),
    })
    
    // Create session with TTL
    session, err := concurrency.NewSession(client, 
        concurrency.WithTTL(int(config.LockTTL.Seconds())))
    
    // Create mutex
    lockKey := filepath.Join(config.LockPrefix, path)
    mutex := concurrency.NewMutex(session, lockKey)
    
    return &EtcdLock{
        client:  client,
        session: session,
        mutex:   mutex,
    }, nil
}
```

## Locks to Implement

### 1. Scheduler Lock
- **Path**: `/scheduler`
- **Purpose**: Ensure only one scheduler runs
- **TTL**: 15 seconds (configurable)
- **Behavior**: Standby schedulers wait for lock

### 2. DAG Run Locks
- **Path**: `/runs/{dagName}/{runID}`
- **Purpose**: Prevent duplicate DAG execution
- **TTL**: 24 hours (or DAG timeout)
- **Behavior**: Fail fast if already locked

### 3. Optional: Queue Operation Lock
- **Path**: `/queue`
- **Purpose**: Atomic queue operations
- **TTL**: 5 seconds
- **Behavior**: Brief lock for enqueue/dequeue

## Minimal Code Changes

### 1. Scheduler Lock
```go
// internal/scheduler/scheduler.go
func (s *Scheduler) Start(ctx context.Context) error {
    // OLD: lock := dirlock.New(schedulerLockPath)
    // NEW:
    lock, err := lock.NewLock("scheduler", s.config)
    if err != nil {
        return err
    }
    
    // Same logic, just better lock
    if err := lock.TryLock(); err != nil {
        return fmt.Errorf("scheduler already running")
    }
    defer lock.Unlock()
    
    // Rest remains the same...
}
```

### 2. DAG Run Lock
```go
// internal/agent/agent.go
func (a *Agent) StartDAGRun(ctx context.Context, dag models.DAG, runID string) {
    // NEW: Use etcd lock instead of dirlock
    lockPath := fmt.Sprintf("runs/%s/%s", dag.Name, runID)
    lock, err := lock.NewLock(lockPath, a.config)
    
    if err := lock.TryLock(); err != nil {
        logger.Warn(ctx, "DAG run already executing")
        return
    }
    defer lock.Unlock()
    
    // Rest remains the same...
}
```

## Advantages of This Approach

1. **Minimal Changes**: Just replace lock implementation
2. **No Data Migration**: All data stays in filesystem
3. **Easy Rollback**: Remove etcd config to use dirlock
4. **Lower Risk**: Proven storage layer unchanged
5. **Fast Implementation**: Days not weeks
6. **Clear Separation**: etcd for locks, filesystem for data

## Failure Modes and Handling

### etcd Unavailable at Startup
- Log error and fail fast
- Admin must fix etcd before starting Dagu
- Clear error message about HA requirements

### etcd Fails During Operation
- Existing locks continue to work (session based)
- New lock attempts fail
- Scheduler continues current work
- Monitor etcd health and alert

### Network Partition
- etcd handles via Raft consensus
- Minority partition loses locks
- Majority partition continues operating
- Automatic recovery when partition heals

## Testing Strategy

### Unit Tests
- Mock etcd client for lock operations
- Test lock acquisition and release
- Test timeout and expiration

### Integration Tests
- Real etcd cluster (docker-compose)
- Multiple Dagu instances
- Failover scenarios
- Network failure simulation

### Manual Testing
1. Start 3 Dagu instances with etcd
2. Verify only one scheduler active
3. Kill active scheduler
4. Verify failover within 5 seconds
5. Test DAG run deduplication

## Monitoring

### Metrics
- `dagu_etcd_lock_acquired_total` - Successful lock acquisitions
- `dagu_etcd_lock_failed_total` - Failed lock attempts  
- `dagu_etcd_lock_held_duration` - How long locks are held
- `dagu_etcd_connection_errors` - etcd connection issues

### Health Check
```json
// GET /api/v1/health
{
  "scheduler": {
    "running": true,
    "isLeader": true,
    "lockBackend": "etcd",
    "etcdHealthy": true
  }
}
```

### Logs
```
INFO  Acquired scheduler lock via etcd
WARN  Failed to acquire DAG run lock: already executing
ERROR etcd connection lost, using existing locks
INFO  Scheduler lock released, shutting down
```

## Migration Path

### Phase 1: Optional Feature (v1.x)
- Add etcd lock support
- Default to dirlock if no etcd config
- Document and test thoroughly

### Phase 2: Recommended for HA (v2.x)
- Promote etcd locks for production HA
- Add migration guide
- Performance benchmarks

### Phase 3: Optimization (Future)
- Add lock wait queuing
- Implement fair lock acquisition
- Advanced etcd features if needed

## Summary

Using etcd purely for locks provides:
- ✅ **3-5 second failover** (vs 15-30 seconds)
- ✅ **No stale locks** (session-based TTL)
- ✅ **Minimal code changes** (just lock implementation)
- ✅ **No data migration** (filesystem storage unchanged)
- ✅ **Easy rollback** (remove etcd config)
- ✅ **Production proven** (etcd is battle-tested)

This approach gives us the benefits of etcd's superior locking without the complexity of full state distribution.