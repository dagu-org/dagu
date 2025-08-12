# Continue On Conditions

The `continueOn` configuration allows workflows to continue execution even when steps encounter failures, specific exit codes, or produce certain outputs. This powerful feature enables resilient workflows that can handle errors gracefully and implement sophisticated control flow patterns.

## Overview

By default, Dagu stops workflow execution when a step fails (returns a non-zero exit code). The `continueOn` configuration overrides this behavior, allowing you to:

- Continue execution after failures
- Handle specific exit codes differently
- React to command output patterns
- Mark steps as successful despite failures
- Build fault-tolerant workflows

## Configuration Fields

The `continueOn` configuration supports the following fields:

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `failure` | boolean | Continue execution when the step fails | `false` |
| `skipped` | boolean | Continue execution when the step is skipped | `false` |
| `exitCode` | array | Continue execution for specific exit codes | `[]` |
| `output` | array | Continue execution when output matches patterns | `[]` |
| `markSuccess` | boolean | Mark the step as successful when conditions are met | `false` |

## Field Details

### `failure`

When set to `true`, the workflow continues even if the step fails with any non-zero exit code.

```yaml
steps:
  - name: optional-cleanup
    command: rm -rf /tmp/cache/*
    continueOn:
      failure: true
```

### `skipped`

When set to `true`, the workflow continues when a step is skipped due to unmet preconditions.

```yaml
steps:
  - name: conditional-task
    command: echo "Processing"
    preconditions:
      - condition: "${ENABLE_FEATURE}"
        expected: "true"
    continueOn:
      skipped: true
```

### `exitCode`

An array of specific exit codes that should not stop the workflow. This is useful when dealing with commands that use non-zero exit codes for non-error conditions.

```yaml
steps:
  - name: check-service
    command: echo "Health check OK"
    continueOn:
      exitCode: [0, 1, 2]  # 0=healthy, 1=warning, 2=maintenance
```

### `output`

An array of patterns to match against the command's stdout output. Supports both literal strings and regular expressions (prefixed with `re:`).

```yaml
steps:
  - name: validate-data
    command: echo "Validating"
    continueOn:
      output:
        - "WARNING"                    # Literal string match (substring)
        - "SKIP"                      # Another literal string
        - "re:^INFO:.*"               # Regex: lines starting with "INFO:"
        - "re:WARN-[0-9]+"            # Regex: WARN- followed by numbers
```

**Pattern Matching Rules:**
- **Literal patterns**: Matched as substrings (e.g., "WARNING" matches "WARNING: Low memory")
- **Regex patterns**: Must start with `re:` prefix (e.g., `re:^ERROR.*`)
- Patterns are matched against each line of **stdout only** (stderr is not checked)
- Matching is case-sensitive

### `markSuccess`

When set to `true`, the step is marked as successful if any of the continue conditions are met, even if it would normally be considered a failure.

```yaml
steps:
  - name: best-effort-optimization
    command: echo "Optimizing"
    continueOn:
      failure: true
      markSuccess: true  # Step shows as successful in UI/logs
```

## Common Patterns

### Optional Steps

For steps that are nice-to-have but not critical:

```yaml
steps:
  - name: cache-warmup
    command: echo "Warming cache"
    continueOn:
      failure: true
      
  - name: main-process
    command: echo "Processing"
```

### Handling Known Exit Codes

When working with tools that use exit codes for non-error states:

```yaml
steps:
  - name: git-diff
    command: git diff --exit-code
    continueOn:
      exitCode: [0, 1]  # 0=no changes, 1=changes exist
      
  - name: process-changes
    command: echo "Handling changes"
```

### Warning Detection

Continue execution but handle warnings differently:

```yaml
steps:
  - name: lint-code
    command: eslint .
    continueOn:
      output: ["WARNING", "re:.*warning.*"]
      exitCode: [0, 1]  # 0=no issues, 1=warnings only
      
  - name: strict-lint
    command: eslint . --max-warnings 0
    continueOn:
      failure: false  # This one must pass
```

### Graceful Degradation

Build workflows that degrade gracefully:

```yaml
steps:
  - name: try-optimal-method
    command: echo "Processing with optimal settings"
    continueOn:
      failure: true
      
  - name: fallback-method
    command: echo "Processing with fallback settings"
    preconditions:
      - condition: "${TRY_OPTIMAL_METHOD_EXIT_CODE}"
        expected: "re:[1-9][0-9]*"  # Only run if previous failed
```

### Complex Output Matching

React to specific output patterns:

```yaml
steps:
  - name: deployment-check
    command: kubectl rollout status deployment/app
    continueOn:
      output:
        - "re:Waiting for.*replicas"
        - "re:deployment.*not found"
        - "Unable to connect"
      exitCode: [1]
      
  - name: handle-deployment-issue
    command: echo "Fixing deployment"
```

## Interaction with Other Features

### With Retry Policies

`continueOn` is evaluated after all retries are exhausted:

```yaml
steps:
  - name: flaky-service
    command: echo "Calling service"
    retryPolicy:
      limit: 3
      intervalSec: 5
    continueOn:
      exitCode: [503]  # Continue if still 503 after retries
```

### With Lifecycle Handlers

Steps that continue on failure still trigger `onFailure` handlers:

```yaml
handlerOn:
  failure:
    command: echo "Logging failure"

steps:
  - name: optional-step
    command: echo "Running optional task"
    continueOn:
      failure: true  # Continues, but failure handler still runs
```

### With Dependencies

Dependent steps see the actual status unless `markSuccess` is used:

```yaml
steps:
  - name: step-a
    command: exit 1
    continueOn:
      failure: true
      markSuccess: false  # Default
      
  - name: step-b
    command: echo "Step A status: failed"
    depends: [step-a]  # Runs because of continueOn
    
  - name: step-c
    command: exit 1
    continueOn:
      failure: true
      markSuccess: true  # Override status
      
  - name: step-d
    command: echo "Step C status: success"
    depends: [step-c]  # Sees step-c as successful
```

## Examples

### Database Migration with Warnings

```yaml
steps:
  - name: run-migration
    command: echo "Running migration"
    continueOn:
      output:
        - "re:WARNING:.*already exists"
        - "re:NOTICE:.*will be created"
      exitCode: [0, 1]  # 1 might indicate warnings
      
  - name: verify-migration
    command: echo "Verifying database"
```

### Multi-Cloud Deployment

```yaml
steps:
  - name: deploy-aws
    command: echo "Deploying to AWS"
    continueOn:
      failure: true  # Continue even if AWS fails
      
  - name: deploy-gcp
    command: echo "Deploying to GCP"
    continueOn:
      failure: true  # Continue even if GCP fails
      
  - name: verify-deployment
    command: echo "Verifying cloud deployment"
    continueOn:
      failure: false  # At least one cloud must be working
```

### Service Health Check

```yaml
steps:
  - name: check-primary
    command: curl -f https://primary.example.com/health
    continueOn:
      exitCode: [0, 22, 7]  # 22=HTTP error, 7=connection failed
      
  - name: check-secondary
    command: curl -f https://secondary.example.com/health
    preconditions:
      - condition: "${CHECK_PRIMARY_EXIT_CODE}"
        expected: "re:[1-9][0-9]*"  # Only if primary failed
```
