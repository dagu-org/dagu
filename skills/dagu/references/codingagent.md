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

- `CLAUDECODE` — If inherited, `claude -p` detects a "nested session" and **hangs indefinitely**.
- `ANTHROPIC_API_KEY` — The parent session may inject a session-scoped key invalid for standalone calls. Unsetting lets `claude -p` use its own auth.

**Only needed for `claude -p` steps.** Other agents are unaffected.

## Pattern 1: Single Agent Step

Use `params` for user-configurable prompts and `env` for defaults (model, agent, system prompt).

```yaml
description: "Run a coding agent with a user-provided prompt"

env:
  - CLAUDE_MODEL: claude-sonnet-4-6

params:
  - PROMPT: "Explain the main function in this project"

steps:
  - id: run_agent
    script: |
      unset CLAUDECODE
      unset ANTHROPIC_API_KEY
      claude -p --model "${CLAUDE_MODEL}" "${PROMPT}"
    output: RESULT
```

## Pattern 2: Multi-Agent Pipeline

Chain agents, passing output via `${step_id.stdout}` file references.

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

**Key technique:** `${step_id.stdout}` is a **file path** to the step's captured stdout. Use `cat "${step_id.stdout}"` to read its content. Use `output:` to capture stdout into a variable for string interpolation.

## Agent Quick Examples

### Claude Code
```bash
claude -p "your prompt"                              # basic
claude -p --model opus "your prompt"                 # model selection
cat file.py | claude -p "Review this code"           # stdin
claude -p --max-turns 5 "your prompt"                # limit turns
```

### OpenAI Codex CLI
```bash
codex exec "your prompt"                             # basic
cat prompt.txt | codex exec -                        # stdin
codex exec --full-auto "your prompt"                 # auto mode
codex exec --skip-git-repo-check "your prompt"       # non-git dirs
```

### Gemini CLI
```bash
gemini -p "your prompt"                              # basic
gemini -p -m gemini-3.1-pro-preview "your prompt"    # model selection
cat data.json | gemini -p "Analyze this data"        # stdin
```

### OpenCode
```bash
opencode run "your prompt"                           # basic
opencode run --model anthropic/claude-sonnet-4-6 "prompt"  # model selection
```

### Aider
```bash
aider -m "Add error handling" --yes-always main.go   # basic (edits files directly)
aider -m "Fix bug" --model claude-sonnet-4-6 --yes-always --no-auto-commits buggy.py
```
Note: Aider's `-m` is `--message` (not `--model`). Capture changes via `git diff` in a subsequent step.

### Kiro CLI
```bash
kiro-cli chat --no-interactive --trust-all-tools "your prompt"
kiro-cli chat --no-interactive --trust-tools read,write,shell "prompt"  # limited tools
```
Note: Auth via `kiro-cli login`. Model selection via `kiro-cli settings chat.defaultModel "model-name"`.

## Tips

1. **Use cheaper models for simple tasks** — Reserve powerful models for complex reasoning; use fast/cheap models for formatting, classification, slug generation, etc.

   | Tier | Claude Code | Codex | Gemini CLI |
   |------|-------------|-------|------------|
   | Cheap/fast | `haiku` | `gpt-5.1-codex-mini` | `gemini-2.0-flash` |
   | Balanced | `sonnet` | `gpt-5.4` | `gemini-2.5-flash` |
   | Most capable | `opus` | `gpt-5.3-codex` | `gemini-3.1-pro-preview` |

2. **Prompt as a parameter** — Expose the core prompt via `params:` so users can customize from UI/CLI without editing the DAG.
3. **env for defaults** — Use `env:` for default model names, agent selection, and system prompts.
4. **Large prompts via stdin** — Pipe file contents via stdin rather than embedding in args to avoid quoting issues and arg length limits.
5. **Temp files for complex input** — When combining multiple sources, write to a temp file and pipe it in.
6. **Working directory matters** — Agents that modify files operate relative to the working dir. Use `working_dir:` or `cd` in the script.
7. **Output capture** — Use `output: VAR_NAME` for variable interpolation; use `${step_id.stdout}` for file-path-based access.
8. **Timeouts** — Set generous `timeout_sec:` (300-600s+) on agent steps to avoid premature kills.
9. **Retry on transient failures** — Add `retry_policy: { limit: 3, interval_sec: 30 }` to handle rate limits and network errors.
