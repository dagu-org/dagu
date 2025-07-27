# etcd-Based High Availability Mode Requirements for Dagu

## Document Status
- **Type**: Feature Requirements
- **Date**: 2025-01-27
- **Status**: Draft
- **Related**: [ha_mode_requirements.md](./ha_mode_requirements.md), [ha_mode_design.md](./ha_mode_design.md)

## Executive Summary

This document proposes using etcd as the distributed coordination service for Dagu's HA mode, replacing the file-based dirlock approach. etcd provides stronger consistency guarantees, better failover detection, and more robust distributed locking mechanisms.

## Why etcd?

### Current File-Based Approach Limitations
1. **Stale Lock Detection**: Relies on timestamp checks with configurable thresholds
2. **Split-Brain Risk**: Brief periods where multiple schedulers might run
3. **Network Storage Dependency**: Requires shared filesystem (NFS, EFS, etc.)
4. **Limited Health Monitoring**: No active health checks, only timestamp-based detection

### etcd Advantages
1. **Active Health Checks**: Session-based leases with automatic expiration
2. **Stronger Consistency**: Raft consensus ensures no split-brain
3. **No Shared Filesystem**: Works across network boundaries
4. **Watch Capabilities**: Real-time notifications of state changes
5. **Proven in Production**: Used by Kubernetes, etcd, and many distributed systems

## Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Dagu #1    │     │  Dagu #2    │     │  Dagu #3    │
│  (Leader)   │     │ (Follower)  │     │ (Follower)  │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │ Leader Election   │ Watch Leader     │ Watch Leader
       │ State Updates     │ State Reads      │ State Reads
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                    ┌──────▼──────┐
                    │    etcd     │
                    │   Cluster   │
                    └─────────────┘
                           │
                    Shared Filesystem
                    (DAGs, Logs, Data)
```

## Config Package Updates

### 1. New Configuration Section

```go
// internal/config/config.go

type Config struct {
    // ... existing fields ...
    HA *HAConfig `mapstructure:"ha"`
}

type HAConfig struct {
    // Enable HA mode
    Enabled bool `mapstructure:"enabled"`
    
    // etcd configuration
    Etcd *EtcdConfig `mapstructure:"etcd"`
    
    // Leader election settings
    Election *ElectionConfig `mapstructure:"election"`
    
    // State synchronization settings
    StateSync *StateSyncConfig `mapstructure:"stateSync"`
}

type EtcdConfig struct {
    // etcd endpoints
    Endpoints []string `mapstructure:"endpoints"`
    
    // Key namespace for this Dagu cluster
    Namespace string `mapstructure:"namespace"`
    
    // Connection settings
    DialTimeout    time.Duration `mapstructure:"dialTimeout"`
    RequestTimeout time.Duration `mapstructure:"requestTimeout"`
    
    // Authentication
    Username string `mapstructure:"username"`
    Password string `mapstructure:"password"`
    
    // TLS configuration
    TLS *TLSConfig `mapstructure:"tls"`
    
    // Connection pool
    MaxCallSendMsgSize int `mapstructure:"maxCallSendMsgSize"`
    
    // Health check
    AutoSyncInterval time.Duration `mapstructure:"autoSyncInterval"`
}

type TLSConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    CertFile string `mapstructure:"certFile"`
    KeyFile  string `mapstructure:"keyFile"`
    CAFile   string `mapstructure:"caFile"`
    
    // Skip verification for testing
    InsecureSkipVerify bool `mapstructure:"insecureSkipVerify"`
}

type ElectionConfig struct {
    // Leader key TTL
    TTL time.Duration `mapstructure:"ttl"`
    
    // How often to renew leader lease
    RenewInterval time.Duration `mapstructure:"renewInterval"`
    
    // Campaign timeout for elections
    CampaignTimeout time.Duration `mapstructure:"campaignTimeout"`
    
    // Instance identification
    InstanceID string `mapstructure:"instanceId"`
}

type StateSyncConfig struct {
    // State update interval
    UpdateInterval time.Duration `mapstructure:"updateInterval"`
    
    // Batch size for bulk operations
    BatchSize int `mapstructure:"batchSize"`
    
    // Retry settings
    MaxRetries     int           `mapstructure:"maxRetries"`
    RetryInterval  time.Duration `mapstructure:"retryInterval"`
}
```

### 2. Configuration File Example

```yaml
# HA Mode Configuration
ha:
  enabled: true
  
  etcd:
    endpoints:
      - "https://etcd1.example.com:2379"
      - "https://etcd2.example.com:2379"
      - "https://etcd3.example.com:2379"
    
    namespace: "/dagu/prod"  # Isolate different Dagu clusters
    
    dialTimeout: 5s
    requestTimeout: 10s
    autoSyncInterval: 30s
    
    # Authentication
    username: ${ETCD_USERNAME}
    password: ${ETCD_PASSWORD}
    
    # TLS Configuration
    tls:
      enabled: true
      certFile: /etc/dagu/certs/etcd-client.crt
      keyFile: /etc/dagu/certs/etcd-client.key
      caFile: /etc/dagu/certs/etcd-ca.crt
    
  election:
    ttl: 15s              # Leader lease TTL
    renewInterval: 5s     # Renew every 5s (1/3 of TTL)
    campaignTimeout: 10s  # Wait up to 10s during election
    instanceId: ${HOSTNAME}-${POD_NAME}  # Kubernetes example
    
  stateSync:
    updateInterval: 1s    # How often to sync state
    batchSize: 100       # Batch operations for efficiency
    maxRetries: 3
    retryInterval: 1s

# Backward compatibility: disable etcd, use file-based
# ha:
#   enabled: false
```

### 3. Environment Variable Support

```bash
# etcd endpoints
DAGU_HA_ETCD_ENDPOINTS="https://etcd1:2379,https://etcd2:2379"

# Authentication
DAGU_HA_ETCD_USERNAME="dagu-service"
DAGU_HA_ETCD_PASSWORD="secure-password"

# TLS paths
DAGU_HA_ETCD_TLS_CERTFILE="/certs/client.crt"
DAGU_HA_ETCD_TLS_KEYFILE="/certs/client.key"
DAGU_HA_ETCD_TLS_CAFILE="/certs/ca.crt"

# Election settings
DAGU_HA_ELECTION_TTL="15s"
DAGU_HA_ELECTION_INSTANCEID="dagu-prod-1"
```

## Key Design Decisions

### 1. Hybrid Approach: etcd + Shared Filesystem
- **etcd**: Coordination, leader election, distributed state
- **Filesystem**: DAG files, logs, historical data
- **Rationale**: Leverages etcd's strengths without major refactoring

### 2. Namespace Isolation
- Each Dagu cluster uses a unique namespace in etcd
- Allows multiple clusters to share etcd infrastructure
- Example: `/dagu/prod`, `/dagu/staging`, `/dagu/dev`

### 3. Hybrid Locking Strategy
- etcd for leader election and distributed state
- File-based locks remain for queue and DAG runs (no harm in keeping)
- If etcd is unavailable, fall back to single-instance mode
- No data loss, just temporary loss of HA

### 4. State Stored in etcd

#### Leader Election
```
/dagu/prod/election/leader -> {
  "instanceId": "dagu-prod-1",
  "startTime": "2025-01-27T10:00:00Z",
  "lastRenewal": "2025-01-27T10:00:05Z"
}
```

#### Scheduler State
```
/dagu/prod/scheduler/state -> {
  "lastProcessedTime": "2025-01-27T10:00:00Z",
  "version": 1
}
```

#### Active DAG Runs
```
/dagu/prod/runs/active/{dagName}/{runID} -> {
  "instanceId": "dagu-prod-1",
  "startTime": "2025-01-27T10:00:00Z",
  "status": "running"
}
```

#### Queue State
```
/dagu/prod/queue/items/{id} -> {
  "dagName": "daily-backup",
  "runID": "20250127-100000",
  "priority": 1,
  "enqueueTime": "2025-01-27T10:00:00Z"
}
```

#### Suspend Flags
```
/dagu/prod/suspend/{dagName} -> {
  "suspended": true,
  "reason": "Manual suspension",
  "timestamp": "2025-01-27T10:00:00Z"
}
```

## Implementation Phases

### Phase 1: Core etcd Integration
1. Add etcd client library dependency
2. Implement configuration loading and validation
3. Create etcd connection manager with health checks
4. Add graceful degradation logic

### Phase 2: Leader Election
1. Implement leader election using etcd session/lease
2. Add leader status to health endpoint
3. Restrict scheduler operations to leader only
4. Add leadership change notifications

### Phase 3: State Synchronization  
1. Migrate scheduler state to etcd
2. Implement distributed queue operations
3. Sync suspend flags across instances
4. Add DAG run locking via etcd

### Phase 4: Advanced Features
1. Watch-based updates for real-time sync
2. Bulk operations for performance
3. Metrics and monitoring
4. Administrative tools

## Comparison with File-Based Approach

| Feature | File-Based (Current Design) | etcd-Based (This Proposal) |
|---------|---------------------------|-------------------------|
| **Dependencies** | Shared filesystem | etcd cluster |
| **Lock Mechanism** | Directory creation | etcd lease/session |
| **Health Detection** | Timestamp staleness | Active session monitoring |
| **Failover Time** | 10-30 seconds | 3-5 seconds |
| **Split-Brain Risk** | Possible during failover | Prevented by Raft |
| **Network Partitions** | May cause issues | Handled by quorum |
| **Operational Complexity** | Low | Medium |
| **Scalability** | Limited by filesystem | High |

## Migration Strategy

### 1. Configuration-Based Mode Selection
```yaml
ha:
  enabled: true
  # Presence of etcd config automatically enables etcd mode
  # Absence of etcd config uses file-based mode
```

### 2. Gradual Rollout
1. Deploy etcd cluster
2. Enable etcd mode on one instance
3. Verify leader election works
4. Enable on remaining instances
5. Monitor for issues

### 3. Rollback Plan
- Remove etcd configuration section
- Instances automatically revert to file-based locking
- No data migration required

## Testing Requirements

### 1. etcd Failure Scenarios
- etcd node failures
- Network partitions
- etcd cluster unavailable
- Slow etcd responses

### 2. Leader Election Tests
- Normal election
- Leader crash
- Network partition
- Multiple candidates

### 3. State Consistency Tests
- Concurrent state updates
- Large state sizes
- Watch event delivery
- Retry mechanisms

### 4. Performance Tests
- Election time
- State sync latency
- etcd load impact
- Failover duration

## Security Considerations

### 1. etcd Access Control
- Use separate etcd user for Dagu
- Limit permissions to Dagu namespace
- Enable etcd RBAC

### 2. TLS Requirements
- Mandatory TLS for production
- Client certificate authentication
- Regular certificate rotation

### 3. Secrets Management
- etcd credentials in environment variables
- Consider secret management systems
- Avoid hardcoding credentials

## Monitoring and Observability

### 1. Metrics
- `dagu_ha_leader` - Current leader gauge
- `dagu_ha_election_duration` - Election time histogram
- `dagu_etcd_request_duration` - etcd operation latency
- `dagu_etcd_errors_total` - etcd error counter

### 2. Health Checks
```go
// GET /health/ha
{
  "enabled": true,
  "backend": "etcd",  // or "file" based on configuration
  "isLeader": true,
  "leaderInstance": "dagu-prod-1",
  "etcdHealth": "healthy",
  "lastLeaderChange": "2025-01-27T10:00:00Z"
}
```

## Open Questions

1. **etcd Deployment**: Should Dagu bundle etcd or require external cluster?
2. **Resource Limits**: Maximum state size in etcd?
3. **Disaster Recovery**: Backup strategy for etcd data?

## Conclusion

Using etcd for Dagu's HA mode provides:
- **Stronger Consistency**: Raft consensus prevents split-brain
- **Faster Failover**: 3-5 second detection vs 15-30 seconds
- **Better Monitoring**: Active health checks and session management
- **Proven Solution**: Battle-tested in Kubernetes and other systems

The main trade-off is increased operational complexity (running etcd cluster) vs improved reliability and faster failover. For production HA deployments, this trade-off is typically worthwhile.