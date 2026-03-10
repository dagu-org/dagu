# Coding Agent Integration

Use AI coding agent CLIs (Claude Code, Codex, Gemini, etc.) as DAG steps for non-interactive, prompt-driven automation.

## Agent CLI Quick Reference

| Agent | Non-interactive command | Stdin | Model flag | API key env var |
|-------|------------------------|-------|------------|-----------------|
| Claude Code | `claude -p "prompt"` | `\| claude -p "prompt"` | `--model` | `ANTHROPIC_API_KEY` |
| Codex | `codex exec "prompt"` | `\| codex exec -` | `-m` / `--model` | `CODEX_API_KEY` |
| Gemini CLI | `gemini -p "prompt"` | `\| gemini -p "prompt"` | `-m` / `--model` | `GEMINI_API_KEY` |
| OpenCode | `opencode run "prompt"` | `\| opencode -p "prompt"` | `-m` / `--model` | Provider-specific |
| Aider | `aider -m "prompt" --yes-always` | `--message-file /dev/stdin` | `--model` | Provider-specific |
| Kiro CLI | `kiro-cli chat --no-interactive --trust-all-tools "prompt"` | N/A | Settings-based | `kiro-cli login` |

## Critical: Nested Session Prevention (Claude Code only)

When Dagu is launched from inside Claude Code, environment variables leak into child steps and cause `claude -p` to hang. **Unset these right before every `claude -p` call:**

```yaml
script: |
  unset CLAUDECODE
  unset ANTHROPIC_API_KEY
  claude -p "your prompt"
```

- `CLAUDECODE` — Claude Code sets this internally. If a child `claude -p` process inherits it, it detects a "nested session" and **hangs indefinitely** instead of running.
- `ANTHROPIC_API_KEY` — The parent Claude Code session may inject a session-scoped API key that is invalid for standalone `claude -p` calls. Unsetting it lets `claude -p` use its own auth (e.g., credentials from `claude login`).

**These only need to be unset for `claude -p` steps.** Other agents (Codex, Gemini, Aider, etc.) do not read these variables and are unaffected.

## Pattern 1: Simple Single-Agent Step

Use `params` to make the prompt user-configurable at run time.

```yaml
description: "Run a coding agent with a user-provided prompt"

params:
  - PROMPT: "Explain the main function in this project"

steps:
  - id: run_agent
    script: |
      unset CLAUDECODE
      claude -p "${PROMPT}"
    output: RESULT
```

## Pattern 2: Configurable Agent via env

Use `env` to define prompt templates and agent configuration that users can override.

```yaml
description: "Code review with configurable agent and prompt"

env:
  - AGENT: claude
  - REVIEW_INSTRUCTIONS: "Review for bugs, security issues, and performance problems."

params:
  - FILE_PATH: ""

steps:
  - id: review
    script: |
      unset CLAUDECODE
      case "${AGENT}" in
        claude)
          cat "${FILE_PATH}" | claude -p "${REVIEW_INSTRUCTIONS}"
          ;;
        codex)
          cat "${FILE_PATH}" | codex exec -
          ;;
        gemini)
          cat "${FILE_PATH}" | gemini -p "${REVIEW_INSTRUCTIONS}"
          ;;
      esac
    output: REVIEW_RESULT
```

## Pattern 3: Multi-Agent Pipeline

Chain different agents, passing output from one to the next via `${step_id.stdout}` file references.

```yaml
description: "Research pipeline: research, review, refine"
type: graph

params:
  - topic: ""

steps:
  - id: research
    description: "Deep research using Claude"
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      claude -p "Research every approach to: ${topic}. List all approaches with pros, cons, and when to use each."
    output: RESEARCH

  - id: review
    description: "Review research for gaps using Codex"
    script: |
      PROMPT_FILE=$(mktemp)
      {
        echo "Review this research for completeness and gaps:"
        echo ""
        cat "${research.stdout}"
      } > "$PROMPT_FILE"
      codex exec --skip-git-repo-check - < "$PROMPT_FILE"
      rm -f "$PROMPT_FILE"
    depends: [research]
    output: REVIEW

  - id: refine
    description: "Refine with review feedback using Claude"
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      {
        echo "=== Research ==="
        cat "${research.stdout}"
        echo ""
        echo "=== Review Feedback ==="
        cat "${review.stdout}"
      } | claude -p "Refine this research incorporating the review feedback provided via stdin."
    depends: [review]
    output: REFINED
```

**Key technique:** `${step_id.stdout}` is a **file path** to the step's captured stdout. Use `cat "${step_id.stdout}"` to read its content. Use the `output:` field to capture stdout content into a variable for use in string interpolation.

## Pattern 4: Parallel Agent Execution

Run multiple agents in parallel on the same task, then combine results.

```yaml
description: "Get multiple perspectives on a code change"
type: graph

params:
  - DIFF: ""

steps:
  - id: claude_review
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      cat "${DIFF}" | claude -p "Review this diff for bugs and improvements."

  - id: gemini_review
    script: |
      cat "${DIFF}" | gemini -p "Review this diff for bugs and improvements."

  - id: combine
    description: "Merge perspectives"
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      {
        echo "=== Claude Review ==="
        cat "${claude_review.stdout}"
        echo ""
        echo "=== Gemini Review ==="
        cat "${gemini_review.stdout}"
      } | claude -p "Synthesize these two code reviews into a unified actionable summary."
    depends: [claude_review, gemini_review]
    output: COMBINED
```

## Pattern 5: Code Generation with File Output

Use coding agents to generate code and write it to disk.

```yaml
description: "Generate a module from a spec"

params:
  - SPEC: "Create a Go HTTP handler that returns system health status as JSON"
  - OUTPUT_DIR: "./generated"

steps:
  - id: generate
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      mkdir -p "${OUTPUT_DIR}"
      cd "${OUTPUT_DIR}"
      claude -p "${SPEC}" --output-format text
    working_dir: "${OUTPUT_DIR}"

  - id: validate
    description: "Validate generated code compiles"
    script: |
      cd "${OUTPUT_DIR}"
      go build ./... 2>&1 || true
    depends: [generate]
```

## Pattern 6: Agent with Model Selection

Use `env` to make the model configurable.

```yaml
env:
  - CLAUDE_MODEL: claude-sonnet-4-6

params:
  - PROMPT: ""

steps:
  - id: run
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      claude -p --model "${CLAUDE_MODEL}" "${PROMPT}"
    output: RESULT
```

## Agent-Specific Usage Details

### Claude Code

```bash
# Basic prompt
claude -p "your prompt"

# With model selection
claude -p --model claude-sonnet-4-6 "your prompt"
claude -p --model opus "your prompt"

# JSON output (structured)
claude -p --output-format json "your prompt"

# Pipe content in via stdin
cat file.py | claude -p "Review this code"

# Add system prompt while keeping defaults
claude -p --append-system-prompt "You are a security auditor" "Audit this code"

# Limit turns to prevent runaway execution (useful for CI/automation)
claude -p --max-turns 5 "your prompt"
```

### OpenAI Codex CLI

```bash
# Basic prompt
codex exec "your prompt"

# Read prompt from stdin (use `-` as the prompt argument)
cat prompt.txt | codex exec -

# With model override
codex exec -m gpt-5.3-codex "your prompt"

# Full auto mode (sets approval=on-request, sandbox=workspace-write)
codex exec --full-auto "your prompt"

# Skip git repo check (important in temp dirs or non-git contexts)
codex exec --skip-git-repo-check "your prompt"

# JSON streaming output (for programmatic consumption)
codex exec --json "your prompt"

# Quiet mode (suppress progress output)
codex exec -q "your prompt"
```

### Gemini CLI

```bash
# Basic prompt
gemini -p "your prompt"

# With model selection
gemini -p -m gemini-3.1-pro-preview "your prompt"

# Pipe content via stdin (auto-detects non-TTY)
cat data.json | gemini -p "Analyze this data"

# Redirect output
gemini -p "Generate a report" > report.md

# JSON output (for headless mode)
gemini -p --output-format json "your prompt"
```

### OpenCode

```bash
# Non-interactive (preferred method)
opencode run "your prompt"

# Legacy non-interactive (still works)
opencode -p "your prompt"

# With model (provider/model format)
opencode run --model anthropic/claude-sonnet-4-6 "your prompt"

# Quiet mode (no spinner, better for scripts)
opencode -p -q "your prompt"

# JSON output
opencode run --format json "your prompt"
```

### Aider

```bash
# Single prompt with auto-confirm (essential for non-interactive)
aider -m "Add error handling to main.go" --yes-always main.go

# Read prompt from file
aider --message-file instructions.txt --yes-always src/*.py

# Disable auto-commits
aider -m "Refactor the auth module" --yes-always --no-auto-commits auth.go

# Specify model
aider -m "Fix the bug" --model claude-sonnet-4-6 --yes-always buggy.py
```

**Note:** Aider's `-m` flag is `--message` (not `--model`). Aider edits files directly rather than printing to stdout. Capture changes via `git diff` in a subsequent step.

### Kiro CLI (formerly Amazon Q Developer CLI)

```bash
# Basic non-interactive (new command)
kiro-cli chat --no-interactive --trust-all-tools "your prompt"

# Legacy command (still works)
q chat --no-interactive --trust-all-tools "your prompt"

# Trust only specific tools
kiro-cli chat --no-interactive --trust-tools read,write,shell "your prompt"
```

**Note:** Authenticate via `kiro-cli login` (supports GitHub, Google, AWS Builder ID, AWS IAM Identity Center, and external identity providers). Model selection is done via settings: `kiro-cli settings chat.defaultModel "claude-opus4.6"` or the interactive `/model` command.

## Tips

1. **Use cheaper models for simple tasks** — Not every step needs the most capable (and expensive) model. Use smaller/cheaper models for trivial tasks like generating a filename slug, formatting text, simple classification, or extracting a value. Reserve powerful models for complex reasoning, multi-file code generation, or deep analysis.
   ```yaml
   steps:
     # Trivial task: cheap model is sufficient
     - id: gen_slug
       script: |
         unset CLAUDECODE
         unset ANTHROPIC_API_KEY
         claude -p --model haiku \
           "Generate a short lowercase filename slug for: ${topic}. Reply with ONLY the slug." \
           | tr -d '[:space:]'

     # Complex task: use a capable model
     - id: deep_analysis
       script: |
         unset CLAUDECODE
         unset ANTHROPIC_API_KEY
         claude -p --model opus "Analyze the architecture of this codebase and suggest improvements."
   ```
   **Model cost tiers (approximate):**
   | Tier | Claude Code | Codex | Gemini CLI |
   |------|-------------|-------|------------|
   | Cheap/fast | `haiku` | `gpt-5.1-codex-mini` | `gemini-2.0-flash` |
   | Balanced | `sonnet` | `gpt-5.4` | `gemini-2.5-flash` |
   | Most capable | `opus` | `gpt-5.3-codex` | `gemini-3.1-pro-preview` |
2. **Prompt as a parameter** — Always expose the core prompt via `params:` so users can customize it from the UI or CLI without editing the DAG.
3. **env for defaults** — Use `env:` (list form) for default model names, agent selection, system prompts, and other knobs users may want to tune.
4. **Large prompts via stdin** — For prompts that include file contents or multi-step instructions, pipe via stdin rather than embedding in command args. This avoids shell quoting issues and argument length limits.
5. **Temp files for complex input** — When combining multiple sources into a single prompt (e.g., research + review), write to a temp file and pipe it in:
   ```yaml
   script: |
     PROMPT_FILE=$(mktemp)
     { echo "Instructions:"; cat "${prev_step.stdout}"; } > "$PROMPT_FILE"
     codex exec - < "$PROMPT_FILE"
     rm -f "$PROMPT_FILE"
   ```
6. **Working directory matters** — Coding agents that modify files (Aider, Claude Code without `-p`) operate relative to the working directory. Use the step-level `working_dir:` field or `cd` in the script.
7. **Output capture** — Use `output: VAR_NAME` to capture agent stdout into a variable for use in subsequent steps via `${VAR_NAME}`. For file-path-based access, use `${step_id.stdout}`.
8. **Timeouts** — LLM calls can be slow. Set generous `timeout_sec:` on agent steps (300–600s or more) to avoid premature kills.
9. **Retry on transient failures** — API rate limits and network errors are common. Add `retry_policy:` to agent steps:
   ```yaml
   retry_policy:
     limit: 3
     interval_sec: 30
   ```
