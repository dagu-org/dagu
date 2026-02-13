---
id: "006"
title: "Variable Expansion Syntax v1"
status: implemented
---

# RFC 006: Variable Expansion Syntax v1

## Summary

This RFC documents the v1 variable expansion system in Dagu. It serves as a reference for understanding existing behavior.

## Syntax Overview

### Variable Reference Patterns

| Syntax | Example | Description |
|--------|---------|-------------|
| `${VAR}` | `${OUTPUT_DIR}` | Braced variable reference |
| `$VAR` | `$HOME` | Unbraced variable reference |
| `$1`, `$2`, `$N` | `$1` | Positional parameters (1-indexed) |
| `${step.stdout}` | `${download.stdout}` | Step stdout output |
| `${step.stderr}` | `${fetch.stderr}` | Step stderr output |
| `${step.exit_code}` | `${check.exit_code}` | Step exit code |
| `${step.exitCode}` | `${check.exitCode}` | Step exit code (alternative) |
| `${VAR:start:len}` | `${uid:0:8}` | String slicing |
| `${VAR.path}` | `${response.data.id}` | JSON path extraction |
| `` `cmd` `` | `` `date +%Y` `` | Command substitution |
| `'${VAR}'` | `'${LITERAL}'` | Single-quoted (preserved literally) |

---

## Parameters

### Supported Formats

**Format 1: String (space-separated)**
```yaml
params: "batch_size=100 environment=prod"
# Or positional only:
params: "value1 value2 value3"
```

**Format 2: List of key-value pairs**
```yaml
params:
  - batch_size: 100
  - environment: prod
```

**Format 3: Map**
```yaml
params:
  batch_size: 100
  environment: prod
```

**Format 4: With JSON Schema validation**
```yaml
params:
  schema: ./params-schema.json
  values:
    batch_size: 100
    environment: prod
```

### Parameter References

| Reference | Description |
|-----------|-------------|
| `$1`, `$2`, `$3` | Positional parameters (1-indexed) |
| `${param_name}` | Named parameters |

### Parameter Evaluation

Parameters are evaluated sequentially, allowing later params to reference earlier ones:

```yaml
params:
  - base_dir: /data
  - output_dir: "${base_dir}/output"  # References base_dir
```

Named parameters are exported as environment variables with their name as the key.

---

## Environment Variables

### Scope Hierarchy

Variables are resolved in this order (highest to lowest precedence):

1. **Step Env** - Step-level `env:` field
2. **Step Outputs** - stdout/stderr/exit_code from previous steps
3. **Secrets** - Secret values
4. **DAG Env** - DAG-level `env:` field (includes dotenv values and named parameters)
5. **OS Environment** - Process environment

Note: Dotenv files are loaded into DAG Env. Named parameters with explicit names are exported as environment variables and become part of DAG Env.

### DAG-Level Environment

```yaml
env:
  OUTPUT_DIR: /tmp/output
  API_URL: https://api.example.com
  # Can reference OS env vars
  HOME_OUTPUT: "${HOME}/output"
```

### Step-Level Environment

```yaml
steps:
  - name: process
    env:
      STEP_VAR: step_value
      # Overrides DAG-level OUTPUT_DIR
      OUTPUT_DIR: /tmp/step_output
    command: echo $STEP_VAR
```

### OS Environment Expansion Rules

OS environment variables are **only expanded in the DAG-level `env:` field** at DAG load time. All other fields pass OS variables through unchanged.

| Field | OS Env Expanded? | Example |
|-------|------------------|---------|
| `env:` (DAG-level) | Yes | `${HOME}` → `/home/user` |
| `secrets:` | No | `key` field is not expanded |
| `command:` | No | `${HOME}` stays as `${HOME}` |
| step `env:` | No | Evaluated at runtime |
| `params:` | No | `${HOME}` stays as `${HOME}` |

**Example:**

```yaml
env:
  # OS env IS expanded here at load time
  OUTPUT_DIR: "${HOME}/output"  # becomes "/home/user/output"

steps:
  - name: example
    # OS env is NOT expanded - shell handles it at runtime
    command: echo $HOME
```

This design allows:
1. DAG configuration to capture OS values at load time when needed
2. Commands to use live OS environment at execution time
3. Clear distinction between Dagu expansion and shell expansion

### Non-Shell Executor Types

For non-shell executor types (docker, http, ssh, mail, jq), **all variables including OS environment are expanded by Dagu** before passing to the executor. This differs from shell commands where OS variables pass through to the shell.

| Executor Type | OS Env Expanded? | Expansion Timing |
|---------------|------------------|------------------|
| `command` (shell) | No | Shell handles at runtime |
| `docker` | Yes | Before container creation |
| `http` | Yes | Before HTTP request |
| `ssh` | Yes | Before SSH connection |
| `mail` | Yes | Before sending email |
| `jq` | Yes | Before query execution |

**Example:**

```yaml
env:
  REGISTRY: myregistry.com

steps:
  # Shell command: $HOME passes through to shell
  - name: shell-step
    command: echo $HOME

  # Docker: $HOME is expanded by Dagu before container runs
  - name: docker-step
    executor:
      type: docker
      config:
        image: "${REGISTRY}/app:latest"  # Expanded to "myregistry.com/app:latest"
        env:
          - "DATA_DIR=$HOME/data"        # $HOME expanded by Dagu

  # HTTP: All variables expanded before request
  - name: http-step
    executor:
      type: http
      config:
        url: "https://api.example.com/${API_VERSION}/data"
        headers:
          Authorization: "Bearer ${API_TOKEN}"

  # SSH: Variables expanded before connection
  - name: ssh-step
    executor:
      type: ssh
      config:
        host: "${REMOTE_HOST}"
        user: "${REMOTE_USER}"
        command: "ls -la"
```

**Why the difference?**
- Shell commands can use shell's native variable expansion at runtime
- Non-shell executors have no built-in variable expansion mechanism, so Dagu must expand all variables before passing configuration to them

---

## Step Outputs

### Available Properties

```yaml
${step_name.stdout}      # Captured standard output
${step_name.stderr}      # Captured standard error
${step_name.exit_code}   # Exit code as string
${step_name.exitCode}    # Alternative syntax for exit code
```

### String Slicing

Extract substrings from step outputs or any variable:

```yaml
${step_name.stdout:0:100}   # First 100 characters
${uid:0:8}                   # First 8 characters of uid
```

Format: `${VAR:start:length}` where:
- `start`: Required, zero-based offset
- `length`: Optional, number of characters to extract

### JSON Value Handling

When a variable contains a JSON string, you can access its fields using jq-compatible path syntax. **The path must start with a dot (`.`)** - direct array indexing like `${VAR[0]}` is not supported.

**Supported Syntax:**

| Syntax | Description | Example |
|--------|-------------|---------|
| `${VAR.field}` | Object field access | `${DATA.name}` |
| `${VAR.a.b.c}` | Nested field access | `${DATA.user.address.city}` |
| `${VAR.field[0]}` | Field then array index | `${DATA.items[0]}` |
| `${VAR.field[0].nested}` | Array element's field | `${DATA.items[0].id}` |

**NOT Supported:**

| Syntax | Why |
|--------|-----|
| `${VAR[0]}` | No dot - direct array index not supported |
| `${VAR[0].field}` | Must start with dot, not bracket |

**Example:**

```yaml
env:
  DATA: '{"name": "test", "items": [{"id": 1}, {"id": 2}]}'

steps:
  - name: fetch
    command: curl -s https://api.example.com/data

  - name: process
    command: |
      echo "Name: ${DATA.name}"            # "test"
      echo "First ID: ${DATA.items[0].id}" # "1"
      echo "API ID: ${fetch.stdout.data.id}"
    depends: fetch
```

**Behavior:**
- Uses jq-compatible query syntax internally
- Path must start with a dot (e.g., `.field`, `.items[0]`)
- If the JSON is invalid, the reference is left unchanged (no error)
- If the path doesn't exist, the reference is left unchanged (no error)
- Result is converted to string representation

---

## Secrets

### Definition

```yaml
secrets:
  - name: API_TOKEN
    provider: file
    key: /path/to/secret.txt

  - name: DB_PASSWORD
    provider: env
    key: DB_PASSWORD_ENV_VAR
```

### Providers

| Provider | Description |
|----------|-------------|
| `file` | Load secret from file contents |
| `env` | Load from environment variable |

### Reference

Secrets are referenced using the same syntax as environment variables:

```yaml
steps:
  - name: deploy
    command: curl -H "Authorization: Bearer ${API_TOKEN}" https://api.example.com
```

### Masking

All secret values are automatically masked in logs and UI output with `*******` placeholder.

---

## Command Substitution

### Backtick Syntax

```yaml
env:
  TODAY: "`date +%Y-%m-%d`"
  COMMIT: "`git rev-parse HEAD`"
  HOSTNAME: "`hostname`"
```

### Behavior

- Executed at **build time** (DAG load)
- Uses shell with current environment
- Trailing whitespace trimmed from output
- Backticks can be escaped with `\`` inside strings

---

## Escape Mechanisms

### Single Quotes

Single-quoted variables are preserved literally and not expanded:

```yaml
steps:
  - name: literal
    # ${VAR} is NOT expanded, passed to shell as-is
    command: echo '${VAR}'
```

### Backtick Escaping

To include a literal backtick in strings, escape with backslash:

```yaml
env:
  MARKDOWN: "Use \`code\` for inline code"
```

---

## Evaluation Timing

### Build Time (DAG Load)

The following are evaluated when the DAG is loaded:

1. DAG-level `env:` field values (including OS env expansion)
2. Parameter values and references
3. Secrets are resolved (but `key` field is not expanded)
4. Command substitution (backticks) in `env:` field

**Note:** OS environment variables are only expanded in the DAG-level `env:` field. Other fields like `command:`, step `env:`, `params:`, and `secrets:` store values as-is without OS env expansion.

### Runtime (Step Execution)

The following are evaluated when each step executes:

1. Step output references (`${step.stdout}`, etc.)
2. OS environment variables in commands (handled by shell)
3. Step-level `env:` field values
4. JSON path extraction from step outputs

---


## Known Limitations

### Ambiguity with Shell Variables

The `${VAR}` syntax is identical to POSIX shell syntax, causing ambiguity:

```yaml
steps:
  - name: example
    command: echo ${HOME}  # Dagu or shell expansion?
```

This is the primary motivation for RFC 005's new syntax.

### No Explicit Context

The v1 syntax provides no way to distinguish between:
- DAG environment variables
- OS environment variables
- Parameters
- Secrets

All use the same `${VAR}` or `$VAR` syntax.

### Missing Variable Behavior

Undefined variables are preserved as-is (not expanded) rather than causing errors. This can lead to silent failures.

### No Default Values

There is no syntax for providing default values for undefined variables.

---

## Complete Example

```yaml
name: example-dag
description: Demonstrates v1 variable expansion

env:
  OUTPUT_DIR: "${HOME}/output"
  TODAY: "`date +%Y-%m-%d`"
  API_URL: https://api.example.com

params:
  - batch_size: 100
  - output_file: "${OUTPUT_DIR}/results_${TODAY}.json"

secrets:
  - name: API_TOKEN
    provider: env
    key: MY_API_TOKEN

steps:
  - name: fetch
    command: |
      curl -s -H "Authorization: Bearer ${API_TOKEN}" \
        "${API_URL}/data?limit=${batch_size}" \
        -o /tmp/data.json

  - name: process
    command: |
      jq '.items[]' /tmp/data.json > ${output_file}
      echo "Processed ${fetch.stdout:0:50}..."
    depends: fetch

  - name: deploy
    # $1 and $2 from command line args
    command: deploy.sh $1 $2 ${output_file}
    depends: process
```

