# Dagu Refactoring Plan

**Version**: 1.0  
**Date**: 2025-11-04  
**Status**: Proposed

## Executive Summary

This document outlines a comprehensive refactoring strategy for the Dagu workflow engine. The plan is based on thorough code analysis and aims to improve maintainability, testability, and code organization while maintaining backward compatibility.

## Current State Analysis

### Codebase Metrics

| Metric | Value |
|--------|-------|
| Total Go Files | 389 |
| Production Files | 214 |
| Test Files | 175 |
| Average Lines/File | 265 |
| Test Coverage | ~82% |
| Total Structs | 306 |
| Total Interfaces | 29 |
| Files > 500 lines | 20+ |
| Files > 800 lines | 9 |

### Build & Quality Status

- ✅ **Build**: Passing (all 2754 tests)
- ✅ **Lint**: Clean (golangci-lint)
- ✅ **Dependencies**: Up to date

## Problem Areas Identified

### 1. Large, Complex Files

#### Critical (>900 lines)
- `internal/core/spec/builder.go` (1949 lines)
  - **Issues**: Monolithic builder, mixed concerns, high complexity
  - **Impact**: Difficult to test, maintain, and extend
  
- `internal/runtime/agent/agent.go` (1019 lines)
  - **Issues**: God object with 16+ fields, handles execution, reporting, socket server
  - **Impact**: High coupling, difficult to test individual components

- `internal/service/frontend/api/v2/dags.go` (969 lines)
  - **Issues**: Large API handler file with multiple responsibilities
  - **Impact**: Hard to navigate, API logic scattered

- `internal/runtime/scheduler.go` (966 lines)
  - **Issues**: Complex state machine, mixed scheduling and execution logic
  - **Impact**: Hard to reason about execution flow

- `internal/service/frontend/api/v2/dagruns.go` (910 lines)
  - **Issues**: Similar to dags.go, handles all DAG run operations
  - **Impact**: Duplicate patterns with v1 API

- `internal/service/frontend/api/v1/dags.go` (898 lines)
  - **Issues**: Legacy API with overlapping v2 functionality
  - **Impact**: Code duplication, maintenance burden

- `internal/runtime/builtin/docker/client.go` (894 lines)
  - **Issues**: Handles all Docker operations in one file
  - **Impact**: Complex Docker logic hard to test

#### High Priority (800-900 lines)
- `internal/service/scheduler/scheduler.go` (841 lines)
  - **Issues**: 25+ struct fields, manages multiple concerns (scheduling, queue, health, zombie detection)
  - **Impact**: High cognitive load, difficult to modify safely

- `internal/runtime/agent/progress_tea.go` (812 lines)
  - **Issues**: Large TUI implementation, complex state management
  - **Impact**: UI logic hard to test and maintain

### 2. Architectural Issues

#### God Objects
- **Agent struct** (16 fields): Manages execution, reporting, socket server, progress display
- **Scheduler struct** (25 fields): Handles scheduling, queue management, health checks, zombie detection, coordination

#### High Coupling
- Runtime layer directly depends on persistence layer
- Service layer tightly coupled to specific implementations
- Limited use of dependency injection

#### Mixed Concerns
- Business logic intertwined with infrastructure code
- Validation logic scattered across builders and handlers
- Error handling inconsistent (1172+ error sites)

### 3. API Layer Issues

- **Duplication**: v1 and v2 APIs share 70%+ similar logic
- **Large handlers**: Single files handle all CRUD operations
- **No middleware**: Common logic (auth, validation) repeated
- **Poor organization**: Feature logic not grouped

### 4. Testing Gaps

- Only 29 interfaces for 306 structs (9.5% interface ratio)
- Difficult to mock concrete dependencies
- Integration tests mixed with unit tests
- Large test files (>500 lines) mirror production issues

## Refactoring Strategy

### Guiding Principles

1. **Incremental**: Small, verifiable changes
2. **Backward Compatible**: No breaking API changes
3. **Test-Driven**: Maintain 82%+ coverage
4. **Performance Neutral**: No regressions
5. **Well-Documented**: Clear migration guides

### Phase 1: Foundation (Weeks 1-2)

**Goal**: Establish patterns and extract core abstractions

#### 1.1 Domain Model Extraction
- [ ] Create `internal/domain` package
- [ ] Extract value objects:
  - `DAGRunID` with validation
  - `DAGName` with naming rules
  - `Status` with state machine
  - `Timestamp` with timezone handling
- [ ] Define domain events for audit trail
- [ ] Create aggregate root interfaces

**Files Affected**: `internal/core/*.go`  
**Risk**: Low (new package)  
**Tests**: Add 20+ unit tests for value objects

#### 1.2 Repository Pattern
- [ ] Define repository interfaces in domain layer
- [ ] Implement adapters in persistence layer
- [ ] Create in-memory implementations for testing
- [ ] Add repository integration tests

**Files Affected**: `internal/persistence/*`, `internal/core/execution/*.go`  
**Risk**: Medium (changes data access patterns)  
**Tests**: Add repository integration tests

#### 1.3 Error Handling Strategy
- [ ] Create `internal/errors` package
- [ ] Define error types (NotFound, Validation, Conflict, Internal)
- [ ] Implement error wrapping with context
- [ ] Add error conversion for HTTP responses

**Files Affected**: All packages  
**Risk**: Low (additive changes)  
**Tests**: Error handling test suite

### Phase 2: API Layer Consolidation (Weeks 3-4)

**Goal**: Reduce duplication and improve API organization

#### 2.1 Extract Common API Logic
- [ ] Create `internal/service/frontend/api/common` package
- [ ] Extract shared handlers:
  - `ListHandler` with pagination
  - `GetHandler` with caching
  - `CreateHandler` with validation
  - `UpdateHandler` with optimistic locking
  - `DeleteHandler` with soft delete
- [ ] Implement middleware:
  - Request validation
  - Error handling
  - Logging and tracing
  - Rate limiting

**Files Affected**: `internal/service/frontend/api/v1/*.go`, `internal/service/frontend/api/v2/*.go`  
**Risk**: Medium (changes API layer)  
**Tests**: API integration tests

#### 2.2 Split Large API Files
- [ ] Split `dags.go` into:
  - `dags_list.go` - List and search
  - `dags_get.go` - Get single DAG
  - `dags_create.go` - Create and validate
  - `dags_update.go` - Update operations
  - `dags_delete.go` - Delete operations
  - `dags_actions.go` - Start, stop, restart
- [ ] Apply same pattern to `dagruns.go`
- [ ] Group by feature, not by HTTP method

**Files Affected**: `internal/service/frontend/api/v2/dags.go`, `internal/service/frontend/api/v2/dagruns.go`  
**Risk**: Low (pure refactoring)  
**Tests**: Existing tests should pass

#### 2.3 API Versioning Strategy
- [ ] Document v1 → v2 migration path
- [ ] Mark v1 as deprecated (keep functional)
- [ ] Create compatibility layer for v1 → v2 adapter
- [ ] Plan v3 API structure

**Files Affected**: Documentation  
**Risk**: Low (no code changes yet)

### Phase 3: Service Layer Decomposition (Weeks 5-7)

**Goal**: Break down god objects into cohesive services

#### 3.1 Scheduler Decomposition
- [ ] Extract from `scheduler.go` (841 lines):
  - `SchedulerCore` - Core scheduling logic (200 lines)
  - `QueueManager` - Queue operations (150 lines)
  - `HealthMonitor` - Health checking (100 lines)
  - `ZombieDetector` - Zombie process detection (100 lines)
  - `CoordinatorClient` - Coordination logic (150 lines)
- [ ] Define interfaces for each component
- [ ] Wire together with dependency injection
- [ ] Add component integration tests

**Files Affected**: `internal/service/scheduler/scheduler.go`  
**Risk**: High (core orchestration logic)  
**Tests**: Comprehensive integration tests required

**Before**:
```go
type Scheduler struct {
    hm runtime.Manager
    er EntryReader
    logDir string
    stopChan chan struct{}
    running atomic.Bool
    location *time.Location
    dagRunStore execution.DAGRunStore
    queueStore execution.QueueStore
    procStore execution.ProcStore
    cancel context.CancelFunc
    lock sync.Mutex
    queueConfigs sync.Map
    pendingMu sync.Mutex
    pendingRuns map[string]int
    config *config.Config
    dirLock dirlock.DirLock
    dagExecutor *DAGExecutor
    healthServer *HealthServer
    serviceRegistry execution.ServiceRegistry
    disableHealthServer bool
    heartbeatCancel context.CancelFunc
    heartbeatDone chan struct{}
    zombieDetector *ZombieDetector
    instanceID string
}
```

**After**:
```go
// Core scheduler with minimal dependencies
type Scheduler struct {
    queue QueueManager
    health HealthMonitor
    zombie ZombieDetector
    coordinator CoordinatorClient
    executor DAGExecutor
    config *SchedulerConfig
}

// Each component is independently testable
type QueueManager interface {
    Enqueue(ctx context.Context, run DAGRun) error
    Dequeue(ctx context.Context) (DAGRun, error)
    GetQueueStatus(ctx context.Context, name string) (QueueStatus, error)
}

type HealthMonitor interface {
    Start(ctx context.Context) error
    Stop() error
    GetHealth() HealthStatus
}

type ZombieDetector interface {
    Start(ctx context.Context) error
    Stop() error
    Scan(ctx context.Context) ([]ZombieRun, error)
}
```

#### 3.2 Agent Decomposition
- [ ] Extract from `agent.go` (1019 lines):
  - `ExecutionAgent` - Core execution (300 lines)
  - `StatusReporter` - Status updates (200 lines)
  - `SocketHandler` - Unix socket server (150 lines)
  - `RetryCoordinator` - Retry logic (150 lines)
  - `ProgressManager` - Progress tracking (150 lines)
- [ ] Create agent builder pattern
- [ ] Add execution middleware/interceptors
- [ ] Implement observer pattern for events

**Files Affected**: `internal/runtime/agent/agent.go`  
**Risk**: High (core execution logic)  
**Tests**: Extensive unit and integration tests

**Before**:
```go
type Agent struct {
    lock sync.RWMutex
    dry bool
    retryTarget *execution.DAGRunStatus
    dagStore execution.DAGStore
    dagRunStore execution.DAGRunStore
    registry execution.ServiceRegistry
    peerConfig config.Peer
    dagRunMgr runtime1.Manager
    scheduler *runtime.Scheduler
    graph *runtime.ExecutionGraph
    reporter *reporter
    socketServer *sock.Server
    logDir string
    logFile string
    dag *core.DAG
    rootDAGRun execution.DAGRunRef
    parentDAGRun execution.DAGRunRef
    dagRunID string
    dagRunAttemptID string
    finished atomic.Bool
    lastErr error
    isSubDAGRun atomic.Bool
    progressDisplay ProgressReporter
    stepRetry string
    tracer *telemetry.Tracer
}
```

**After**:
```go
// Focused execution agent
type ExecutionAgent struct {
    executor Executor
    reporter Reporter
    progress ProgressManager
    config *AgentConfig
}

// Clean interfaces for each concern
type Executor interface {
    Execute(ctx context.Context, dag *core.DAG) error
    Stop(ctx context.Context) error
    Retry(ctx context.Context, step string) error
}

type Reporter interface {
    ReportStart(ctx context.Context, run DAGRun) error
    ReportProgress(ctx context.Context, status Status) error
    ReportComplete(ctx context.Context, result Result) error
}

type ProgressManager interface {
    Start(ctx context.Context) error
    Update(status Status) error
    Stop() error
}
```

#### 3.3 Runtime Scheduler Simplification
- [ ] Refactor `runtime/scheduler.go` (966 lines)
- [ ] Extract state machine into separate component
- [ ] Create execution strategy pattern
- [ ] Implement pipeline for node execution
- [ ] Add execution hooks for observability

**Files Affected**: `internal/runtime/scheduler.go`  
**Risk**: High (core runtime)  
**Tests**: State machine tests, integration tests

### Phase 4: Builder Pattern Refactoring (Weeks 8-9)

**Goal**: Simplify the 1949-line builder.go

#### 4.1 Chain of Responsibility
- [ ] Split `builder.go` into:
  - `builder_core.go` - Main builder orchestration
  - `builder_metadata.go` - Metadata builders (env, schedule, params)
  - `builder_config.go` - Config builders (working dir, container, ssh)
  - `builder_steps.go` - Step builders
  - `builder_handlers.go` - Handler builders (mail, preconditions)
  - `builder_validation.go` - Validation logic
- [ ] Create builder chain processor
- [ ] Each builder validates its own section
- [ ] Add builder registry for extensibility

**Files Affected**: `internal/core/spec/builder.go`  
**Risk**: Medium (complex but well-tested)  
**Tests**: Builder test suite should pass

**Before**:
```go
// 1949 lines in one file with ~100 builder functions
func buildEnvs(ctx BuildContext, spec *definition, dag *core.DAG) error { ... }
func buildSchedule(ctx BuildContext, spec *definition, dag *core.DAG) error { ... }
func buildParams(ctx BuildContext, spec *definition, dag *core.DAG) error { ... }
// ... 97 more functions
```

**After**:
```go
// builder_core.go - Orchestration
type BuilderChain struct {
    builders []Builder
}

func (c *BuilderChain) Build(ctx BuildContext, spec *definition) (*core.DAG, error) {
    dag := &core.DAG{}
    for _, builder := range c.builders {
        if err := builder.Build(ctx, spec, dag); err != nil {
            return nil, err
        }
    }
    return dag, nil
}

// builder_metadata.go - Grouped by concern
type MetadataBuilder struct {}
func (b *MetadataBuilder) Build(ctx BuildContext, spec *definition, dag *core.DAG) error {
    if err := b.buildEnv(ctx, spec, dag); err != nil { return err }
    if err := b.buildSchedule(ctx, spec, dag); err != nil { return err }
    if err := b.buildParams(ctx, spec, dag); err != nil { return err }
    return nil
}

// builder_config.go - Runtime configuration
type ConfigBuilder struct {}
func (b *ConfigBuilder) Build(ctx BuildContext, spec *definition, dag *core.DAG) error {
    // Build working dir, container, ssh configs
    return nil
}
```

#### 4.2 Validation Extraction
- [ ] Create `internal/core/spec/validation` package
- [ ] Extract validators:
  - `DAGValidator` - Overall DAG validation
  - `StepValidator` - Step validation
  - `DependencyValidator` - Dependency graph validation
  - `ScheduleValidator` - Cron expression validation
  - `ParameterValidator` - Parameter validation
- [ ] Implement validation rules engine
- [ ] Add validation error aggregation

**Files Affected**: `internal/core/spec/builder.go`, new package  
**Risk**: Low (extraction)  
**Tests**: Validation test suite

### Phase 5: Docker Integration Refactoring (Week 10)

**Goal**: Simplify Docker client (894 lines)

#### 5.1 Docker Client Decomposition
- [ ] Split into:
  - `client.go` - Core client (200 lines)
  - `images.go` - Image operations (200 lines)
  - `containers.go` - Container lifecycle (200 lines)
  - `network.go` - Network operations (100 lines)
  - `auth.go` - Registry authentication (100 lines)
- [ ] Create Docker operation interfaces
- [ ] Add Docker client mock for testing
- [ ] Implement retry and timeout strategies

**Files Affected**: `internal/runtime/builtin/docker/client.go`  
**Risk**: Medium (external dependency)  
**Tests**: Docker integration tests (with test containers)

### Phase 6: Testing Infrastructure (Weeks 11-12)

**Goal**: Improve testability and increase coverage to 90%

#### 6.1 Interface Extraction
- [ ] Extract interfaces for major components:
  - Identify all concrete dependencies
  - Create interfaces in domain layer
  - Update constructors to accept interfaces
  - Generate mocks with `mockgen`
- [ ] Target: 60+ interfaces (from 29)

**Files Affected**: All packages  
**Risk**: Medium (widespread changes)  
**Tests**: Update all tests to use mocks

#### 6.2 Test Fixtures Factory
- [ ] Create `internal/test/fixtures` package
- [ ] Implement builder pattern for test data
- [ ] Add fixtures for:
  - DAGs (simple, complex, with dependencies)
  - DAG runs (success, failure, retry)
  - Steps (all executor types)
  - Configurations
- [ ] Create fixture randomization for property testing

**Files Affected**: New package  
**Risk**: Low (test-only)  
**Tests**: Fixture tests

#### 6.3 Table-Driven Tests
- [ ] Convert large test functions to table-driven
- [ ] Extract common test patterns
- [ ] Create test helpers for:
  - HTTP API testing
  - Database testing
  - File system testing
  - Time-dependent testing
- [ ] Document testing patterns in TESTING.md

**Files Affected**: All `*_test.go` files  
**Risk**: Low (test improvements)  
**Tests**: Meta tests for test helpers

### Phase 7: UI Code Organization (Weeks 13-14)

**Goal**: Improve frontend structure and maintainability

#### 7.1 Component Organization
- [ ] Restructure UI directories:
  ```
  src/
    features/           # Feature-based organization
      dags/
        components/     # DAG-specific components
        hooks/          # DAG-specific hooks
        api/            # DAG API calls
        types/          # DAG types
      dag-runs/
      queues/
      workers/
    shared/
      components/       # Shared UI components
      hooks/            # Shared hooks
      utils/            # Utilities
    core/
      api/              # API client
      config/           # Configuration
      types/            # Shared types
  ```
- [ ] Extract 20+ shared hooks
- [ ] Create component library

**Files Affected**: `ui/src/*`  
**Risk**: Medium (UI refactoring)  
**Tests**: Component tests, E2E tests

#### 7.2 State Management
- [ ] Implement consistent state management pattern
- [ ] Create custom hooks for:
  - Data fetching
  - Form handling
  - Modal management
  - Toast notifications
- [ ] Add optimistic updates
- [ ] Implement request caching

**Files Affected**: `ui/src/*`  
**Risk**: Medium (changes data flow)  
**Tests**: Hook tests

#### 7.3 TypeScript Improvements
- [ ] Generate TypeScript types from OpenAPI spec
- [ ] Create type-safe API client
- [ ] Add strict null checks
- [ ] Implement discriminated unions for state
- [ ] Add runtime type validation with Zod

**Files Affected**: `ui/src/*`  
**Risk**: Low (type safety improvements)  
**Tests**: Type tests

### Phase 8: Cross-Cutting Concerns (Weeks 15-16)

**Goal**: Standardize logging, metrics, and observability

#### 8.1 Structured Logging
- [ ] Standardize log levels and format
- [ ] Add structured context to all logs
- [ ] Implement log sampling for high-volume operations
- [ ] Create logging middleware for HTTP and gRPC
- [ ] Add correlation IDs across services

**Files Affected**: All packages  
**Risk**: Low (observability improvement)  
**Tests**: Log output validation

#### 8.2 Metrics and Tracing
- [ ] Add OpenTelemetry metrics:
  - Request latency
  - DAG execution time
  - Queue depth
  - Error rates
  - Resource usage
- [ ] Implement distributed tracing
- [ ] Add custom metrics for business events
- [ ] Create Grafana dashboards

**Files Affected**: All packages  
**Risk**: Low (observability)  
**Tests**: Metrics tests

#### 8.3 Circuit Breaker and Retry
- [ ] Implement circuit breaker for:
  - External service calls
  - Database operations
  - File system operations
- [ ] Standardize retry strategies
- [ ] Add exponential backoff with jitter
- [ ] Implement timeout contexts consistently

**Files Affected**: Service layer  
**Risk**: Medium (changes error handling)  
**Tests**: Resilience tests

### Phase 9: Documentation (Week 17)

**Goal**: Comprehensive documentation for maintainers

#### 9.1 Architecture Documentation
- [ ] Create architecture decision records (ADRs)
- [ ] Document system architecture with C4 models
- [ ] Add sequence diagrams for key flows
- [ ] Create component interaction diagrams
- [ ] Document data flows

**Files Affected**: `docs/architecture/`  
**Risk**: None (documentation)

#### 9.2 Development Guides
- [ ] Update CONTRIBUTING.md with new patterns
- [ ] Create TESTING.md with testing strategies
- [ ] Add REFACTORING_GUIDE.md for future refactorings
- [ ] Document common pitfalls and solutions
- [ ] Create debugging guides

**Files Affected**: `docs/development/`  
**Risk**: None (documentation)

#### 9.3 API Documentation
- [ ] Generate OpenAPI specs from code
- [ ] Add API examples for all endpoints
- [ ] Document error codes and responses
- [ ] Create API migration guides (v1 → v2 → v3)
- [ ] Add Postman/Insomnia collections

**Files Affected**: `docs/api/`  
**Risk**: None (documentation)

### Phase 10: Technical Debt (Week 18)

**Goal**: Address accumulated technical debt

#### 10.1 Code Cleanup
- [ ] Address all TODO comments (8 identified)
- [ ] Remove deprecated code marked in v1.0
- [ ] Clean up unused imports
- [ ] Remove dead code (uncalled functions)
- [ ] Consolidate duplicate code

**Files Affected**: All packages  
**Risk**: Low (cleanup)  
**Tests**: Ensure existing tests pass

#### 10.2 Dependency Updates
- [ ] Update all dependencies to latest stable
- [ ] Replace deprecated dependencies
- [ ] Remove unused dependencies (25 go.mod)
- [ ] Audit security vulnerabilities
- [ ] Document dependency rationale

**Files Affected**: `go.mod`, `ui/package.json`  
**Risk**: Medium (dependency updates)  
**Tests**: Full regression test suite

#### 10.3 Performance Optimization
- [ ] Profile hot paths
- [ ] Optimize database queries
- [ ] Add caching where appropriate
- [ ] Reduce memory allocations
- [ ] Benchmark critical paths

**Files Affected**: Various  
**Risk**: Medium (performance changes)  
**Tests**: Performance benchmarks

## Implementation Guidelines

### Code Review Checklist

For each refactoring PR:

- [ ] Changes are incremental and focused
- [ ] All existing tests pass
- [ ] New tests added for new code
- [ ] Test coverage maintained/improved
- [ ] No performance regressions (benchmarks)
- [ ] Documentation updated
- [ ] API compatibility maintained
- [ ] Lint checks pass
- [ ] No new security vulnerabilities
- [ ] Changes reviewed by 2+ engineers

### Testing Strategy

1. **Unit Tests**: Test individual components in isolation
2. **Integration Tests**: Test component interactions
3. **Contract Tests**: Test API contracts
4. **E2E Tests**: Test critical user journeys
5. **Performance Tests**: Benchmark hot paths
6. **Chaos Tests**: Test resilience

### Rollout Strategy

1. **Feature Flags**: Enable gradual rollout
2. **Canary Deployments**: Test in production with small subset
3. **Monitoring**: Watch metrics closely
4. **Rollback Plan**: Quick rollback capability
5. **Documentation**: Migration guides for users

## Risk Management

### High-Risk Changes

| Change | Risk | Mitigation |
|--------|------|------------|
| Scheduler Decomposition | High | Extensive integration tests, gradual rollout |
| Agent Refactoring | High | Parallel implementation, feature flag |
| API Changes | Medium | Versioning, deprecation notices |
| Database Schema | Medium | Migrations with rollback scripts |

### Rollback Procedures

1. Keep previous version deployable
2. Database migrations must be reversible
3. Feature flags for major changes
4. Automated rollback triggers on error rate spike

## Success Metrics

### Code Quality

- [ ] Average file size: 265 → <200 lines
- [ ] Files >500 lines: 20 → <10
- [ ] Cyclomatic complexity: Reduce by 30%
- [ ] Test coverage: 82% → 90%
- [ ] Interface ratio: 9.5% → 20%

### Performance

- [ ] No regression in DAG execution time
- [ ] API latency p99 maintained
- [ ] Memory usage stable or improved
- [ ] Startup time maintained

### Developer Experience

- [ ] Build time maintained (<2 min)
- [ ] Test execution time <5 min
- [ ] Clear component boundaries
- [ ] Easy to add new features
- [ ] Comprehensive documentation

## Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1: Foundation | 2 weeks | None |
| Phase 2: API Layer | 2 weeks | Phase 1 |
| Phase 3: Service Layer | 3 weeks | Phase 1 |
| Phase 4: Builder Pattern | 2 weeks | Phase 1 |
| Phase 5: Docker Integration | 1 week | Phase 1 |
| Phase 6: Testing | 2 weeks | Phases 2-5 |
| Phase 7: UI | 2 weeks | Phase 2 |
| Phase 8: Cross-Cutting | 2 weeks | Phases 2-5 |
| Phase 9: Documentation | 1 week | All phases |
| Phase 10: Tech Debt | 1 week | All phases |

**Total Duration**: ~18 weeks (4.5 months)

## Conclusion

This refactoring plan provides a structured approach to improving the Dagu codebase. By following these phases incrementally, we can:

1. Reduce complexity and improve maintainability
2. Increase test coverage and reliability
3. Improve developer productivity
4. Enable faster feature development
5. Maintain backward compatibility

The plan is ambitious but achievable with dedicated effort and adherence to the guiding principles. Each phase builds upon previous phases, creating a solid foundation for future development.

## Appendix

### A. Detailed File Analysis

See [File Analysis Report](./docs/refactoring/file-analysis.md) for detailed breakdown of each file.

### B. Architecture Diagrams

See [Architecture Diagrams](./docs/refactoring/architecture/) for current and proposed architectures.

### C. Migration Guides

See [Migration Guides](./docs/refactoring/migrations/) for API and code migration examples.

### D. References

- [AGENTS.md](./AGENTS.md) - Development guidelines
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Contribution guide
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
