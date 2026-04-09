# Coding Agent Integration

Use `type: harness` to run AI coding agent CLIs as DAG steps. The harness executor spawns the CLI as a subprocess in non-interactive mode.

## Supported Providers

| Provider | Binary | CLI invocation |
|----------|--------|----------------|
| `claude` | `claude` | `claude -p "<prompt>" [flags]` |
| `codex` | `codex` | `codex exec "<prompt>" [flags]` |
| `copilot` | `copilot` | `copilot -p "<prompt>" [flags]` |
| `opencode` | `opencode` | `opencode run "<prompt>" [flags]` |
| `pi` | `pi` | `pi -p "<prompt>" [flags]` |

The selected attempt's binary must be resolvable when it runs. Built-in providers use `PATH`; custom harnesses can use a binary name or an explicit path resolved from the step working directory.

## How Config Works

Harness supports built-in providers and named custom harness definitions:

- `config.provider` selects a built-in provider such as `claude`, `codex`, or `copilot`
- top-level `harnesses.<name>` defines how to invoke a custom harness CLI
- `config.provider` can point at either a built-in provider or a custom `harnesses:` entry

All non-reserved config keys are passed directly as CLI flags:

- `key: "value"` → `--key value`
- `key: true` → `--key`
- `key: false` → omitted
- `key: 123` → `--key 123`
- built-in providers also normalize `snake_case` keys to kebab-case flags, so `max_turns` becomes `--max-turns`

Reserved keys are `provider` and `fallback`.

`provider` may be parameterized with `${...}` and is resolved at runtime after interpolation.

## Custom Harness Registry

Define reusable custom harness adapters once at the DAG level:

```yaml
harnesses:
  gemini:
    binary: gemini
    prefix_args: ["run"]
    prompt_mode: flag
    prompt_flag: --prompt
    option_flags:
      model: --model

steps:
  - name: review
    type: harness
    command: "Review the current branch"
    config:
      provider: gemini
      model: gemini-2.5-pro
```

Custom harness definition fields:

- `binary` — CLI binary or path
- `prefix_args` — args that always appear before prompt placement and runtime flags
- `prompt_mode` — `arg`, `flag`, or `stdin`
- `prompt_flag` — required when `prompt_mode: flag`
- `prompt_position` — `before_flags` or `after_flags`
- `flag_style` — `gnu_long` or `single_dash`
- `option_flags` — per-option override from config key to exact flag token

## DAG-Level Defaults and Fallback

Use top-level `harness:` to define shared defaults for every harness step in the DAG.

```yaml
harness:
  provider: claude
  model: sonnet
  bare: true
  fallback:
    - provider: codex
      full-auto: true
    - provider: copilot
      yolo: true
      silent: true

steps:
  - name: step1
    command: "Write tests"

  - name: step2
    command: "Fix bugs"
    config:
      model: opus
      effort: high

  - name: step3
    type: harness
    command: "Generate docs"
    config:
      provider: copilot
      fallback:
        - provider: claude
          model: haiku
```

Merge rules:

- DAG-level primary config is the base
- Step-level config overlays it
- Step-level `fallback` replaces DAG-level `fallback`
- If a step omits `type:` and the DAG defines `harness:`, the step is inferred as `type: harness`

## Pattern 1: Single Agent Step

```yaml
params:
  - PROMPT: "Explain the main function in this project"

harness:
  provider: claude
  model: sonnet
  bare: true

steps:
  - name: run_agent
    command: "${PROMPT}"
    output: RESULT
```

## Pattern 2: Multi-Agent Pipeline

Chain agents, passing output between steps via `output:` variables or `${step_id.stdout}` file references.

```yaml
type: graph

params:
  - topic: ""

steps:
  - id: research
    name: research
    type: harness
    command: "Research every approach to: ${topic}. List all approaches with pros, cons, and when to use each."
    config:
      provider: claude
      model: sonnet
      bare: true
    output: RESEARCH

  - id: review
    name: review
    type: harness
    # Interpolated before execution, then piped to the harness CLI on stdin.
    script: |
      Review this research for completeness and gaps:

      ${RESEARCH}
    command: "Review the research provided on stdin for completeness and gaps"
    config:
      provider: codex
      full-auto: true
      skip-git-repo-check: true
    depends: [research]
    output: REVIEW

  - id: refine
    name: refine
    type: harness
    script: |
      === Research ===
      ${RESEARCH}

      === Review Feedback ===
      ${REVIEW}
    command: "Refine this research incorporating the review feedback provided via stdin."
    config:
      provider: claude
      model: sonnet
      bare: true
    depends: [review]
    output: REFINED
```

`command:` is the prompt. For built-in providers and custom `arg`/`flag` harnesses, `script:` is piped to stdin as supplementary context. For custom `stdin` harnesses, stdin receives the prompt, then a blank line, then the script when both are present.

## Pattern 3: Parameterized

```yaml
params:
  - PROVIDER: claude
  - MODEL: sonnet
  - PROMPT: "Analyze this codebase"

steps:
  - name: agent
    type: harness
    command: "${PROMPT}"
    config:
      provider: "${PROVIDER}"
      model: "${MODEL}"
    output: RESULT
```

## Provider Examples

### Claude Code

```yaml
steps:
  - name: task
    type: harness
    command: "Write tests for the auth module"
    config:
      provider: claude
      model: sonnet
      effort: high
      max-turns: 20
      max-budget-usd: 2.00
      permission-mode: auto
      allowed-tools: "Bash,Read,Edit"
      bare: true
    timeout_sec: 300
    output: RESULT
```

### Codex

```yaml
steps:
  - name: task
    type: harness
    command: "Fix failing tests in src/"
    config:
      provider: codex
      full-auto: true
      sandbox: workspace-write
      ephemeral: true
      skip-git-repo-check: true
    timeout_sec: 300
```

### Copilot

```yaml
steps:
  - name: task
    type: harness
    command: "Refactor the authentication middleware"
    config:
      provider: copilot
      autopilot: true
      yolo: true
      silent: true
      no-ask-user: true
      no-auto-update: true
    timeout_sec: 300
```

### OpenCode

```yaml
steps:
  - name: task
    type: harness
    command: "Refactor the database layer"
    config:
      provider: opencode
      format: json
    timeout_sec: 300
```

### Pi

```yaml
steps:
  - name: task
    type: harness
    command: "Design a rate limiting middleware"
    config:
      provider: pi
      thinking: high
      tools: read,bash
    timeout_sec: 300
```

## Notes

1. **Model names** — Look up current model names from each provider's documentation. Do not rely on hardcoded names; they change frequently.
2. **Prompt as a parameter** — Expose the prompt via `params:` so users can customize from UI/CLI without editing the DAG.
3. **Timeouts** — Set `timeout_sec:` (300-600s+) on agent steps. Agent CLIs can run for minutes.
4. **Retry on transient failures** — Add `retry_policy: { limit: 3, interval_sec: 30 }` to handle rate limits and network errors.
5. **Working directory** — Use `working_dir:` on the step. The CLI operates relative to this directory.
6. **Output capture** — Use `output: VAR_NAME` for variable interpolation; use `${step_id.stdout}` for file-path-based access.
7. **Exit codes** — 0 = success, 1 = CLI error, 124 = step timed out. Last 1KB of stderr is included in the error message on failure.
8. **Fallback behavior** — If the primary harness config fails and the context is still active, fallback configs are tried in order. Failed-attempt stdout is discarded; stderr remains visible in logs.
