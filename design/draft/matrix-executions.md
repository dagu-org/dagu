# Matrix Execution Requirements for Dagu

## Overview

This document outlines the requirements for implementing matrix execution functionality in Dagu, allowing users to run workflows with all combinations of multiple parameter sets (similar to GitHub Actions matrix strategy).

## Core Concept

Matrix execution creates a Cartesian product of parameter values, executing the workflow for each combination.

## Basic Syntax

```yaml
steps:
  - name: test-all-combinations
    run: workflows/test
    matrix:
      env: [dev, staging, prod]
      region: [us-east-1, eu-west-1]
      version: [1.0, 2.0]
    # Creates 3×2×2 = 12 executions
```

## Detailed Requirements

### 1. Matrix Definition

#### 1.1 Simple Arrays
```yaml
matrix:
  os: [ubuntu, macos, windows]
  python: [3.9, 3.10, 3.11]
# Results in 3×3 = 9 combinations
```

#### 1.2 Include/Exclude Patterns
```yaml
matrix:
  os: [ubuntu, macos, windows]
  python: [3.9, 3.10, 3.11]
  include:
    # Add specific combinations
    - os: ubuntu
      python: 3.12-rc
      experimental: true
  exclude:
    # Remove specific combinations
    - os: macos
      python: 3.9
    - os: windows
      python: 3.9
```

#### 1.3 Dynamic Matrix from Previous Step
```yaml
steps:
  - name: get-test-matrix
    command: |
      echo '{
        "os": ["ubuntu", "macos"],
        "version": ["stable", "beta"]
      }'
    output: TEST_MATRIX
    
  - name: run-tests
    run: workflows/test
    matrix: ${TEST_MATRIX}
```

### 2. Parameter Access

Matrix values are available as parameters in the execution:

```yaml
steps:
  - name: deploy-matrix
    run: workflows/deploy
    matrix:
      env: [dev, prod]
      region: [us-east-1, eu-west-1]
    params:
      # Matrix values automatically available
      # ${env} and ${region} are set for each execution
      - DEPLOYMENT_NAME: ${env}-${region}
```

### 3. Concurrency Control

```yaml
matrix:
  values:
    env: [dev, staging, prod]
    region: [us-east-1, eu-west-1]
  maxConcurrent: 4  # Default: 10
  failFast: true    # Default: false, stop all on first failure
```

### 4. Output Handling

```yaml
steps:
  - name: matrix-build
    run: workflows/build
    matrix:
      os: [linux, macos]
      arch: [amd64, arm64]
    output: BUILD_RESULTS
    
  - name: aggregate-results
    command: |
      echo "Total builds: ${BUILD_RESULTS.length}"
      echo "Successful: ${BUILD_RESULTS.successful}"
    depends: matrix-build
```

Each matrix execution result includes matrix values:
```json
{
  "matrixValues": {"os": "linux", "arch": "amd64"},
  "output": "build output here",
  "status": "success"
}
```

### 5. Advanced Features

#### 5.1 Conditional Matrix
```yaml
steps:
  - name: check-changes
    command: git diff --name-only HEAD^ HEAD
    output: CHANGED_FILES
    
  - name: test-if-needed
    run: workflows/test
    matrix:
      component: [api, web, worker]
    preconditions:
      # Only test components that changed
      - condition: echo "${CHANGED_FILES}" | grep -q "${component}/"
```

#### 5.2 Matrix with Complex Objects
```yaml
matrix:
  deployment:
    - name: api
      port: 8080
      replicas: 3
    - name: web
      port: 3000
      replicas: 2
  environment: [staging, prod]
# Access: ${deployment.name}, ${deployment.port}, ${environment}
```

#### 5.3 Nested Matrix Execution
```yaml
# Parent DAG
steps:
  - name: deploy-regions
    run: workflows/deploy-region
    matrix:
      region: [us-east-1, eu-west-1, ap-south-1]

# Child DAG (deploy-region.yaml)
steps:
  - name: deploy-services
    run: workflows/deploy-service
    matrix:
      service: [api, web, worker]
    params:
      - REGION: ${REGION}  # From parent
```

### 6. UI Representation

1. **Matrix Grid View**: Show all combinations in a grid
   ```
   ┌─────────┬──────────┬──────────┬──────────┐
   │         │ us-east-1│ eu-west-1│ ap-south │
   ├─────────┼──────────┼──────────┼──────────┤
   │ dev     │ ✓        │ ✓        │ ✓        │
   │ staging │ ✓        │ ⏳       │ ⏸        │
   │ prod    │ ✗        │ ⏸        │ ⏸        │
   └─────────┴──────────┴──────────┴──────────┘
   ```

2. **Progress Tracking**: "6/12 completed (1 failed)"

3. **Filter/Search**: Filter by matrix values

## Comparison with Parallel Execution

| Feature | Parallel | Matrix |
|---------|----------|---------|
| Purpose | Run same task with different data | Test all combinations |
| Input | Array of items | Multiple parameter arrays |
| Combinations | One item at a time | Cartesian product |
| Use Case | Process list of files | Test multiple environments |

## Examples

### CI/CD Testing
```yaml
name: test-suite
steps:
  - name: run-tests
    run: |
      docker run --rm \
        -e TEST_ENV=${env} \
        ${os}:${version} \
        pytest tests/
    matrix:
      os: [ubuntu, alpine]
      version: [20.04, 22.04]
      env: [unit, integration]
    maxConcurrent: 6
```

### Multi-Cloud Deployment
```yaml
name: multi-cloud-deploy
steps:
  - name: provision
    run: terraform apply -auto-approve
    matrix:
      provider: [aws, gcp, azure]
      environment: [dev, staging, prod]
      region: [us, eu, asia]
    exclude:
      # Don't deploy dev to all regions
      - environment: dev
        region: [eu, asia]
    env:
      - CLOUD_PROVIDER: ${provider}
      - ENVIRONMENT: ${environment}
      - REGION: ${region}
```

### Database Migration
```yaml
name: migrate-databases
steps:
  - name: backup-first
    run: backup.sh ${database} ${environment}
    matrix:
      database: [users, products, orders]
      environment: [staging, prod]
    
  - name: migrate
    run: migrate.sh ${database} ${environment}
    matrix:
      database: [users, products, orders]
      environment: [staging, prod]
    depends: backup-first
    # Only runs after all backups complete
```

## Implementation Phases

### Phase 1: Basic Matrix
- Simple array-based matrix
- Automatic parameter injection
- Basic concurrency control

### Phase 2: Advanced Features
- Include/exclude patterns
- Complex object support
- failFast option
- Matrix-aware output aggregation

### Phase 3: Enterprise Features
- Dynamic matrix from previous steps
- Conditional matrix execution
- Matrix visualization in UI
- Performance optimizations

## Edge Cases

1. **Empty Matrix**: Skip if any dimension is empty
2. **Single Dimension**: Works like parallel execution
3. **Large Matrices**: Warn if >100 combinations
4. **Duplicate Combinations**: After include/exclude, remove duplicates
5. **Matrix in Parallel**: Can combine with parallel execution

## Success Metrics

1. Users can easily test multiple combinations
2. Clear visualization of matrix execution
3. Efficient resource usage with concurrent limits
4. Intuitive parameter access in workflows

## Future Considerations

1. **Smart Scheduling**: Prioritize failed combinations in reruns
2. **Partial Matrix**: Run only specific combinations on demand
3. **Matrix Templates**: Reusable matrix definitions
4. **Cost Estimation**: Estimate time/resources before execution