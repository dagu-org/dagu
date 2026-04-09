# Coding Agent Integration

Use `type: harness` to run AI coding agent CLIs as DAG steps. The harness executor spawns the CLI as a subprocess in non-interactive mode.

## Supported Providers

| Provider | Binary | CLI invocation | API key env var |
|----------|--------|----------------|-----------------|
| `claude` | `claude` | `claude -p "<prompt>"` | `ANTHROPIC_API_KEY` |
| `codex` | `codex` | `codex exec "<prompt>"` | `CODEX_API_KEY` |
| `opencode` | `opencode` | `opencode run "<prompt>"` | Provider-specific |
| `pi` | `pi` | `pi -p "<prompt>"` | Provider-specific (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) |

The binary must be pre-installed in `PATH`. If missing, the step fails at setup time.

## Pattern 1: Single Agent Step

```yaml
params:
  - PROMPT: "Explain the main function in this project"

steps:
  - name: run_agent
    type: harness
    command: "${PROMPT}"
    config:
      provider: claude
      model: sonnet
      bare: true
    output: RESULT
```

## Pattern 2: Multi-Agent Pipeline

Chain agents, passing output between steps via `${step_id.stdout}` file references or `output:` variables.

```yaml
description: "Research pipeline: research, review, refine"
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
    script: |
      Review this research for completeness and gaps:

      ${RESEARCH}
    command: "Review the research provided on stdin for completeness and gaps"
    config:
      provider: codex
      effort: high
      skip_git_repo_check: true
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

`command:` is the prompt passed to the CLI flag. `script:` content is piped to the CLI's stdin as supplementary context.

## Pattern 3: Parameterized Model Selection

```yaml
params:
  - PROVIDER: claude
  - MODEL: sonnet
  - EFFORT: high
  - PROMPT: "Analyze this codebase"

steps:
  - name: agent
    type: harness
    command: "${PROMPT}"
    config:
      provider: "${PROVIDER}"
      model: "${MODEL}"
      effort: "${EFFORT}"
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
      max_turns: 20
      max_budget_usd: 2.00
      permission_mode: auto
      allowed_tools: "Bash,Read,Edit"
      bare: true
    timeout_sec: 300
    output: RESULT
```

### OpenAI Codex

```yaml
steps:
  - name: task
    type: harness
    command: "Fix failing tests in src/"
    config:
      provider: codex
      model: gpt-5.4
      effort: high
      sandbox: workspace-write
      ephemeral: true
      skip_git_repo_check: true
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
      model: anthropic/claude-sonnet-4-20250514
      output_format: json
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
      pi_provider: anthropic
      model: claude-sonnet-4-20250514
      thinking: high
      tools: read,bash
    timeout_sec: 300
```

## Effort Mapping

The `effort` field is translated differently per provider:

| Effort | Claude | Codex | OpenCode | Pi |
|--------|--------|-------|----------|-----|
| `low` | `--effort low` | (no effect) | (no effect) | `--thinking low` |
| `medium` | `--effort medium` | (no effect) | (no effect) | `--thinking medium` |
| `high` | `--effort high` | `--full-auto` | (no effect) | `--thinking high` |
| `max` | `--effort max` | `--full-auto` | (no effect) | `--thinking xhigh` |

## Stdin Piping

If the step has a `script:` field, its content is piped to the CLI's stdin. The `command:` field is always the prompt (passed via the CLI's prompt flag).

```yaml
steps:
  - name: review
    type: harness
    command: "Review this code for security issues"
    script: |
      func handleLogin(w http.ResponseWriter, r *http.Request) {
          username := r.FormValue("username")
          query := fmt.Sprintf("SELECT * FROM users WHERE name = '%s'", username)
          db.Query(query)
      }
    config:
      provider: claude
      model: sonnet
```

## extra_flags Escape Hatch

For CLI flags not yet modeled in config, use `extra_flags`:

```yaml
steps:
  - name: task
    type: harness
    command: "Summarize the project"
    config:
      provider: claude
      extra_flags:
        - "--verbose"
        - "--no-session-persistence"
```

## Notes

1. **Model tiers** — Use cheaper models for simple tasks, reserve expensive models for complex reasoning.

   | Tier | Claude | Codex |
   |------|--------|-------|
   | Cheap/fast | `haiku` | `gpt-5.1-codex-mini` |
   | Balanced | `sonnet` | `gpt-5.4` |
   | Most capable | `opus` | `gpt-5.3-codex` |

2. **Prompt as a parameter** — Expose the prompt via `params:` so users can customize from UI/CLI without editing the DAG.
3. **Timeouts** — Set `timeout_sec:` (300-600s+) on agent steps. Agent CLIs can run for minutes.
4. **Retry on transient failures** — Add `retry_policy: { limit: 3, interval_sec: 30 }` to handle rate limits and network errors.
5. **Working directory** — Use `working_dir:` on the step or `dir:` at the DAG level. The CLI operates relative to this directory.
6. **Output capture** — Use `output: VAR_NAME` for variable interpolation; use `${step_id.stdout}` for file-path-based access.
7. **Exit codes** — 0 = success, 1 = CLI error, 124 = step timed out. Last 1KB of stderr is included in the error message on failure.
