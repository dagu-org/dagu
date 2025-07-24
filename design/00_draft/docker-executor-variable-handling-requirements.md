# Docker Executor Variable Handling Specification

## Overview

This specification defines how the Docker executor in Dagu should handle environment variables, including variable expansion, precedence rules, and script support. The goal is to achieve feature parity with the command executor while maintaining Docker-specific functionality.

## Motivation

Currently, the Docker executor has several limitations compared to the command executor:

1. Environment variables defined in DAG files are not expanded before being passed to containers
2. No support for the `script` field
3. Inconsistent variable precedence rules
4. Variables like `${VAR}` and command substitution with backticks are not processed
5. Exec mode doesn't properly inherit environment variables

## Specification

### 1. Environment Variable Handling

#### 1.1 Variable Collection

The Docker executor MUST collect all environment variables from the execution context using the `AllEnvs(ctx)` function before creating or executing in containers.

```go
allEnvs := executor.AllEnvs(ctx)
```

#### 1.2 Variable Precedence

Environment variables MUST follow this precedence order (highest to lowest):

1. **Container-specific environment variables** - Defined in `executor.config.container.env`
2. **Exec-specific environment variables** - Defined in `executor.config.exec.env` (for exec mode)
3. **Step-level environment variables** - Defined at the step level
4. **DAG runtime variables** - Parameters passed at runtime
5. **DAG-level environment variables** - Defined in the DAG's `env` section
6. **System environment variables** - From the host system
7. **Container image default variables** - Built into the Docker image

#### 1.3 Variable Merging Algorithm

When merging environment variables:

```go
// Create a map to handle precedence
envMap := make(map[string]string)

// Add variables in order of precedence (lowest to highest)
// 1. System and DAG context variables
for _, env := range allEnvs {
    if key, value, ok := parseEnvVar(env); ok {
        envMap[key] = value
    }
}

// 2. Container/Exec specific variables (highest precedence)
for _, env := range containerConfig.Env {
    if key, value, ok := parseEnvVar(env); ok {
        envMap[key] = value
    }
}

// Convert back to slice format
finalEnvs := make([]string, 0, len(envMap))
for k, v := range envMap {
    finalEnvs = append(finalEnvs, k+"="+v)
}
```

### 2. Variable Expansion

#### 2.1 Supported Expansion Types

The Docker executor MUST support the following variable expansion types:

1. **Simple variables**: `$VAR` or `${VAR}`
2. **Command substitution**: `` `command` `` (backticks)
3. **JSON path references**: `${VAR.field.subfield}`
4. **Environment variable expansion**: Using `os.ExpandEnv`

#### 2.2 Expansion Locations

Variable expansion MUST be performed in:

1. Container environment variables
2. Exec environment variables
3. Working directory paths
4. User specifications
5. Command and arguments

### 3. Script Support

#### 3.1 Script Field Handling

The Docker executor MUST support the `script` field similar to the command executor:

```yaml
steps:
  - name: script-example
    executor:
      type: docker
      config:
        image: python:3.11
    script: |
      #!/usr/bin/env python
      import os
      print(f"Variable: {os.environ.get('MY_VAR')}")
```

#### 3.2 Script Implementation

1. Create a temporary script file in the host system
2. Mount the script file into the container at a predictable location
3. Execute the script with appropriate permissions
4. Clean up the temporary file after execution

```go
// Script handling pseudocode
if step.Script != "" {
    scriptFile := createTempScript(step.Script)
    defer os.Remove(scriptFile)
    
    // Add bind mount
    hostConfig.Binds = append(hostConfig.Binds, 
        fmt.Sprintf("%s:/tmp/dagu_script:ro", scriptFile))
    
    // Override command
    containerConfig.Cmd = []string{"/bin/sh", "/tmp/dagu_script"}
}
```

### 4. Exec Mode Enhancements

#### 4.1 Environment Variable Inheritance

When executing in an existing container, the executor MUST:

1. Collect all environment variables from the execution context
2. Merge them with exec-specific environment variables
3. Pass the merged environment to the exec command

#### 4.2 Working Directory Expansion

The working directory in exec options MUST support variable expansion:

```go
if execOptions.WorkingDir != "" {
    execOptions.WorkingDir = expandVariables(ctx, execOptions.WorkingDir)
}
```

### 5. Configuration Examples

#### 5.1 New Container with Environment Variables

```yaml
env:
  - BASE_URL: https://api.example.com
  - API_KEY: ${SECRET_KEY}

steps:
  - name: api-call
    executor:
      type: docker
      config:
        image: alpine:latest
        container:
          env:
            - ENDPOINT: ${BASE_URL}/users
            - TIMEOUT: "30"
    command: wget -O- "$ENDPOINT"
```

#### 5.2 Exec Mode with Variables

```yaml
params:
  - OPERATION: backup

steps:
  - name: maintenance
    executor:
      type: docker
      config:
        containerName: database
        exec:
          env:
            - OPERATION_TYPE: ${OPERATION}
          workingDir: /var/lib/data
          user: postgres
    command: ./maintenance.sh
```

#### 5.3 Script with Variable Expansion

```yaml
env:
  - DATA_DIR: /data
  - PROCESS_DATE: "`date +%Y-%m-%d`"

steps:
  - name: process
    executor:
      type: docker
      config:
        image: python:3.11
        container:
          workingDir: ${DATA_DIR}
    script: |
      #!/usr/bin/env python
      import os
      date = os.environ.get('PROCESS_DATE')
      print(f"Processing data for {date}")
```

### 6. Error Handling

1. If variable expansion fails, the executor SHOULD log a warning but continue execution
2. If script creation fails, the executor MUST return an error
3. If environment variable parsing fails, the malformed variable SHOULD be skipped

### 7. Backward Compatibility

1. Existing DAGs without variable references MUST continue to work unchanged
2. Container-specific environment variables MUST always take precedence
3. The default behavior MUST not break existing workflows

### 8. Testing Requirements

#### 8.1 Unit Tests

1. Test environment variable merging with correct precedence
2. Test variable expansion in different contexts
3. Test script file creation and cleanup
4. Test exec mode environment handling

#### 8.2 Integration Tests

1. Test complete DAG execution with docker executor and variables
2. Test variable precedence in real scenarios
3. Test script execution in containers
4. Test exec mode with inherited variables

### 9. Implementation Notes

1. Reuse existing variable expansion functions from `cmdutil` package
2. Follow the same patterns as the command executor where applicable
3. Ensure thread safety when handling temporary script files
4. Log all variable expansions at debug level for troubleshooting

## Acceptance Criteria

1. Docker executor supports all environment variable features available in command executor
2. Variable precedence is clearly defined and consistently applied
3. Script support works reliably across different container images
4. Exec mode properly inherits and expands environment variables
5. All existing DAGs continue to work without modification
6. Comprehensive test coverage for all new functionality