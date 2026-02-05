---
id: "007"
title: "OS Environment Variable Expansion Rules"
status: implemented
amends: "006"
---

# RFC 007: OS Environment Variable Expansion Rules

**Amends:** [RFC 006 — Variable Expansion Syntax v1](./006-variable-expansion-v1.md)

## Summary

This RFC improves the v1 variable expansion system documented in RFC 006. Specifically, it changes the OS environment variable expansion rules for non-shell executors so that OS variables are **only expanded when explicitly imported** into the DAG's scope. Variables not defined in the DAG (env, params, secrets, step outputs) are left as-is and passed through to the target execution environment. This fixes a class of bugs where Dagu silently replaces variables the user intended for a remote or container environment.

## Motivation

### The Problem

RFC 006 documents that for non-shell executors (docker, http, ssh, mail, jq), **all variables including OS environment are expanded by Dagu** before passing to the executor (see RFC 006, "Non-Shell Executor Types" section). This breaks user intent when the variable is meant for the target environment, not the local machine.

**SSH example (broken today):**

```yaml
steps:
  - name: remote-backup
    executor:
      type: ssh
      config:
        host: myserver.com
        user: deploy
        command: "tar czf $HOME/backup.tar.gz /data"
```

The user intends `$HOME` to resolve on **the remote machine** (e.g., `/home/deploy`). Instead, Dagu expands it locally (e.g., `/Users/alice`) before sending the command over SSH, producing `tar czf /Users/alice/backup.tar.gz /data` on the remote host.

**Docker example (broken today):**

```yaml
steps:
  - name: container-task
    executor:
      type: docker
      config:
        image: python:3.12
        env:
          - "WORKDIR=$HOME/app"
```

The user may want the container's `$HOME` (e.g., `/root`), but Dagu substitutes the host machine's `$HOME` before creating the container.

### Why This Happens

The `EnvScope` chain resolves variables through layers: StepEnv > Outputs > Secrets > DAGEnv > **OS**. The OS layer is the fallback at the bottom of the chain. When a variable like `$HOME` is not defined anywhere in the DAG, the scope falls through to the OS environment and expands it.

The relevant code path:

1. `node.setupExecutor()` (`internal/runtime/node.go:396`) calls `EvalString()` / `EvalObject()` on all executor config fields
2. `cmdutil.EvalString()` (`internal/cmn/cmdutil/eval.go:154`) performs variable expansion via `expandVariables()`
3. `expandVariables()` uses `EnvScope.Expand()` (`internal/cmn/cmdutil/envscope.go:189`) which walks the full scope chain including OS environment
4. Non-shell executors use default `EvalOptions` where `ExpandEnv=true`

Shell commands avoid this by registering `WithoutExpandEnv()` in their `GetEvalOptions()` callback, letting the shell handle `$VAR` at runtime. Non-shell executors have no equivalent protection.

### Scope of Impact

| Executor | Impact | Common broken patterns |
|----------|--------|----------------------|
| **ssh** | High | `$HOME`, `$USER`, `$PATH` on remote host |
| **docker** | High | Container env vars like `$HOME`, `$HOSTNAME` |
| **http** | Low | Rarely uses OS-like variable names in URLs/headers |
| **mail** | Low | Variable references in email templates |
| **jq** | Low | Variables in jq query strings |

---

## Proposal

### New Rule: Expand Only What Is Defined

Dagu should only expand variables that exist within its own scope:

- **DAG-level `env:`** fields
- **Step-level `env:`** fields
- **Params** (named and positional)
- **Secrets**
- **Step outputs** (`stdout`, `stderr`, `exitCode`)
- **Dagu built-in variables** (`DAG_NAME`, `DAG_RUN_ID`, etc.)

If a variable reference (`$VAR` or `${VAR}`) does not match any of these, it is **left as-is** rather than falling through to the OS environment.

### Explicit OS Import

Users who need an OS environment variable must explicitly import it via the DAG-level `env:` field:

```yaml
env:
  # Explicitly import OS vars into DAG scope
  HOME: "${HOME}"
  REGISTRY: "${REGISTRY}"

steps:
  - name: remote-backup
    executor:
      type: ssh
      config:
        host: myserver.com
        command: "tar czf $HOME/backup.tar.gz /data"
        # $HOME is NOT expanded - remote shell resolves it

  - name: docker-build
    executor:
      type: docker
      config:
        image: "${REGISTRY}/app:latest"
        # ${REGISTRY} IS expanded - defined in DAG env
```

### Behavior Comparison

| Variable | Defined in DAG? | Current behavior | Proposed behavior |
|----------|----------------|-----------------|-------------------|
| `${OUTPUT_DIR}` | Yes (in `env:`) | Expanded | Expanded (no change) |
| `${HOME}` | No | Expanded from OS | **Left as-is** |
| `${HOME}` | Yes (imported in `env:`) | Expanded | Expanded (no change) |
| `${step.stdout}` | Yes (step output) | Expanded | Expanded (no change) |
| `${API_KEY}` | Yes (in `secrets:`) | Expanded | Expanded (no change) |
| `${batch_size}` | Yes (in `params:`) | Expanded | Expanded (no change) |
| `${UNKNOWN}` | No | Expanded (empty string or OS value) | **Left as-is** |

### What "Left As-Is" Means

When a variable is not found in the DAG scope, the literal text is preserved in the output string:

- `$HOME` stays as `$HOME`
- `${PATH}` stays as `${PATH}`

For SSH and Docker, this means the target environment's shell or runtime resolves the variable. For HTTP and other executors, this means the literal text appears in the request (which is almost certainly what the user wants if they wrote it).

---

## Technical Design

### 1. Modify EnvScope Resolution

**File: `internal/cmn/cmdutil/envscope.go`**

Add a `skipOS` flag to `EnvScope` that prevents fallthrough to the OS environment layer when resolving variables. The OS layer is still present in the chain (for DAG-level `env:` evaluation at load time), but is skipped during step-level expansion for non-shell executors.

```go
type EnvScope struct {
    // existing fields...
    skipOS bool // When true, do not resolve from OS environment
}
```

Add a constructor option or method to create a scope view that skips OS:

```go
func (s *EnvScope) WithoutOSFallback() *EnvScope {
    // Return a new scope with skipOS=true
    // This scope still sees DAGEnv, StepEnv, Outputs, Secrets
    // but does NOT fall through to the OS environment layer
}
```

### 2. Secrets Are Not Affected

**Files: `internal/cmn/secrets/env.go`, `internal/runtime/agent/agent.go`**

Secret resolution is a **separate code path** from variable expansion. The `env` provider calls `os.LookupEnv()` directly to read the OS environment variable specified by the `key` field. This happens in `agent.resolveSecrets()` early in `Agent.Run()`, before any step execution begins.

Once resolved, secret values are added to the `EnvScope` with `EnvSourceSecret` source. During step-level expansion, secrets are found in the scope as explicitly defined entries (not OS-sourced), so they expand normally.

```yaml
secrets:
  - name: DB_PASSWORD
    provider: env              # os.LookupEnv("MY_DB_PASSWORD") - unaffected by this RFC
    key: MY_DB_PASSWORD

steps:
  - name: query
    executor:
      type: ssh
      config:
        host: db-server
        command: "psql -p ${DB_PASSWORD} -U $USER"
        # ${DB_PASSWORD} → expanded (resolved secret, in scope)
        # $USER → left as-is (OS-only, not in scope → remote shell resolves it)
```

The same applies to `provider: file` — it reads the file directly, independent of `EnvScope`.

### 3. Dotenv Files Are Not Affected

**Files: `internal/core/dag.go`, `internal/core/exec/context.go`**

Dotenv values are loaded via `godotenv.Read()` in `dag.loadSingleDotEnvFile()` (`dag.go:376-402`) and **appended directly to `dag.Env`** as `KEY=VALUE` strings. At runtime, they enter the `EnvScope` tagged as `EnvSourceDAGEnv` — the same source as variables explicitly written in the `env:` block.

From the expansion engine's perspective, dotenv variables are part of the DAG's explicit scope and will always be expanded, regardless of this RFC's change.

```yaml
# .env file:
#   DATABASE_URL=postgres://db-host:5432/mydb
#   API_KEY=abc123

dotenv: .env

steps:
  - name: migrate
    executor:
      type: ssh
      config:
        host: db-server
        command: "DATABASE_URL=${DATABASE_URL} migrate up"
        # ${DATABASE_URL} → expanded (loaded from .env into DAG scope)
```

Loading order in `Agent.Run()`:
1. `LoadDotEnv()` — dotenv values appended to `dag.Env`
2. `resolveSecrets()` — secrets resolved (can reference dotenv values)
3. Step execution — both dotenv and secret values are in scope

### 4. Change Non-Shell Executor Expansion

**File: `internal/runtime/node.go`**

In `setupExecutor()`, when evaluating executor config fields for non-shell executors, use a scope that excludes OS environment fallback:

```go
func (n *Node) setupExecutor(ctx context.Context) error {
    // For non-shell executors, use scope without OS fallback
    if !n.isShellExecutor() {
        ctx = withScopeWithoutOS(ctx)
    }
    // ... existing evaluation code ...
}
```

### 5. Preserve DAG-Level OS Expansion

**File: `internal/core/spec/variables.go`**

No change needed. The DAG-level `env:` field evaluation already uses OS environment at load time (via `evaluatePairs()` at line 76-86). This is where explicit OS imports like `HOME: "${HOME}"` get resolved. This behavior is preserved.

### 6. Preserve Shell Command Behavior

**File: `internal/runtime/builtin/command/command.go`**

No change needed. Shell commands already use `WithoutExpandEnv()` and pass environment variables through the shell's native `cmd.Env` array. Shell handles `$VAR` at runtime.

### 6a. Preserve Command-Without-Shell Behavior

When a command step has no shell available (rare — only when the system default
shell is unset), there is no target environment to resolve `$VAR` at runtime.
The process receives literal `$HOME` in argv with no shell to expand it.
In this case, the command executor explicitly opts into OS expansion so Dagu
resolves variables on behalf of the missing shell.

### 7. Update SSH Executor

**File: `internal/runtime/builtin/ssh/ssh.go`**

The SSH executor's `GetEvalOptions()` currently only conditionally disables shell expansion. With this change, the scope-based approach handles OS env exclusion automatically, so no executor-specific changes are needed. The executor receives config fields with OS vars left as-is, which the remote shell then resolves.

---

## Examples

### Before (Current Behavior)

```yaml
steps:
  - name: remote-deploy
    executor:
      type: ssh
      config:
        host: prod-server
        user: deploy
        command: |
          cd $HOME/app
          git pull
          ./restart.sh
```

**Result:** `$HOME` is expanded to the local machine's home directory (e.g., `/Users/alice`), causing the remote command to `cd` to a non-existent path.

### After (Proposed Behavior)

```yaml
steps:
  - name: remote-deploy
    executor:
      type: ssh
      config:
        host: prod-server
        user: deploy
        command: |
          cd $HOME/app
          git pull
          ./restart.sh
```

**Result:** `$HOME` is left as `$HOME`. The remote shell resolves it to the deploy user's home directory on `prod-server` (e.g., `/home/deploy`).

### Mixed: Some Dagu Vars, Some Remote Vars

```yaml
env:
  DEPLOY_BRANCH: main
  REMOTE_HOST: prod-server

steps:
  - name: remote-deploy
    executor:
      type: ssh
      config:
        host: "${REMOTE_HOST}"      # Expanded: defined in DAG env
        user: deploy
        command: |
          cd $HOME/app              # NOT expanded: left for remote shell
          git checkout ${DEPLOY_BRANCH}  # Expanded: defined in DAG env
          ./restart.sh
```

### Docker: Container vs Host Variables

```yaml
env:
  REGISTRY: myregistry.com
  APP_VERSION: "2.1.0"

steps:
  - name: run-app
    executor:
      type: docker
      config:
        image: "${REGISTRY}/app:${APP_VERSION}"  # Expanded: both defined in DAG env
        env:
          - "CONFIG_DIR=$HOME/.config"           # NOT expanded: $HOME is the container's
        command: ["./start.sh"]
```

---

## Migration

### Breaking Change Assessment

This is a **behavioral change** that could break existing DAGs that rely on implicit OS environment expansion in non-shell executor configs. However:

1. **Most affected patterns are already broken.** If a user writes `$HOME` in an SSH command, they almost certainly want the remote `$HOME`, not the local one. The current behavior is the bug.
2. **The fix is simple.** Users who genuinely need a local OS variable in executor config just add it to `env:`.
3. **Shell commands are not affected.** The most common executor type (`command:`) is unchanged.

### Migration Path

1. **Add a deprecation warning.** When Dagu expands an OS-only variable (not defined in DAG scope) inside a non-shell executor config, log a warning:
   ```
   WARN: OS variable '${HOME}' expanded in ssh executor config.
         This will stop working in a future version.
         To keep this behavior, add 'HOME: "${HOME}"' to your env: block.
   ```
2. **Introduce the new behavior behind a flag** (optional). A DAG-level `expandMode: strict` field could opt into the new behavior before it becomes the default.
3. **Make it the default** in the next minor release.

### Alternative: No Deprecation Period

Given that the current behavior is almost always wrong for the affected use cases (SSH `$HOME`, Docker `$HOME`), a direct switch could be justified. The warning-first approach is more conservative.

---

## Relationship to Other RFCs

### RFC 006 (Variable Expansion Syntax v1)

This RFC directly amends RFC 006. Specifically, it revises the "Non-Shell Executor Types" section which states that all variables including OS environment are expanded for non-shell executors. After this RFC, the rule becomes: **only variables defined in the DAG's scope are expanded**; OS-only variables pass through unchanged.

All other behavior documented in RFC 006 remains unchanged:
- DAG-level `env:` field still expands OS variables at load time (for explicit import)
- Shell commands still pass `$VAR` through to the shell
- Step outputs, secrets, params, command substitution — all unchanged
- Scope hierarchy (StepEnv > Outputs > Secrets > DAGEnv > OS) unchanged for shell commands

The updated non-shell executor table from RFC 006 becomes:

| Executor Type | DAG-scoped vars expanded? | OS env expanded? |
|---------------|--------------------------|------------------|
| `command` (shell) | No (shell handles) | No (shell handles) |
| `command` (no shell) | Yes | **Yes** (no change) |
| `docker` | Yes | **No** (was: Yes) |
| `http` | Yes | **No** (was: Yes) |
| `ssh` | Yes | **No** (was: Yes) |
| `mail` | Yes | **No** (was: Yes) |
| `jq` | Yes | **No** (was: Yes) |

### RFC 005 (Variable Expansion Syntax Refactoring)

RFC 005 proposes a new `${{ context.VAR }}` syntax with explicit contexts (`sys.HOME`, `env.OUTPUT_DIR`, etc.). That RFC solves the ambiguity problem at the syntax level by requiring users to specify where a variable comes from.

This RFC (007) solves the immediate behavioral problem with the **existing v1 syntax**. The two RFCs are complementary:

- **RFC 007** can ship independently as a bugfix/improvement to v1 behavior
- **RFC 005** is a larger syntax evolution that makes the intent even more explicit
- When RFC 005 ships, the problem addressed here becomes impossible by design (since `${{ sys.HOME }}` is explicit and `$HOME` passes through)

---

## Testing Strategy

### Unit Tests

**`internal/cmn/cmdutil/envscope_test.go`**

```go
func TestEnvScopeWithoutOSFallback(t *testing.T) {
    // Create scope with DAGEnv containing OUTPUT_DIR
    // Create scope.WithoutOSFallback()
    // Verify: ${OUTPUT_DIR} expands (in DAG scope)
    // Verify: ${HOME} does NOT expand (OS only, not in DAG scope)
    // Verify: ${PATH} does NOT expand (OS only)
}
```

### Integration Tests

```go
func TestSSHExecutorNoOSExpansion(t *testing.T) {
    dag := `
env:
  REMOTE_HOST: localhost
steps:
  - name: test
    executor:
      type: ssh
      config:
        host: "${REMOTE_HOST}"
        command: "echo $HOME"
`
    // Verify: host is expanded to "localhost"
    // Verify: command contains literal "$HOME", not local home dir
}

func TestDockerExecutorNoOSExpansion(t *testing.T) {
    dag := `
env:
  REGISTRY: myregistry.com
steps:
  - name: test
    executor:
      type: docker
      config:
        image: "${REGISTRY}/app:latest"
        env:
          - "WORKDIR=$HOME/app"
`
    // Verify: image is expanded to "myregistry.com/app:latest"
    // Verify: env contains literal "$HOME/app"
}

func TestExplicitOSImportStillWorks(t *testing.T) {
    dag := `
env:
  HOME: "${HOME}"
steps:
  - name: test
    executor:
      type: ssh
      config:
        host: localhost
        command: "echo ${HOME}"
`
    // Verify: ${HOME} is expanded (explicitly imported into DAG env)
}
```

---

## Summary of Changes

| Component | Change |
|-----------|--------|
| `EnvScope` | Add `WithoutOSFallback()` that skips OS layer during resolution |
| `node.setupExecutor()` | Use OS-excluded scope for non-shell executor evaluation |
| Shell command executor | No change (already skips Dagu expansion) |
| DAG-level `env:` loading | No change (OS expansion at load time preserved) |
| Non-shell executors | No executor-specific changes needed; scope handles it |
| Documentation | Update variable expansion docs to reflect new rule |
