---
name: AI Workflow Patterns
description: Agent and chat step configuration for LLM-powered DAG workflows
version: 1.0.0
author: Dagu
tags:
  - ai
  - llm
  - agent
  - chat
---
# AI Workflow Patterns

## Chat Steps

Use `type: chat` for LLM-powered steps that process text, answer questions, or generate content.

### Basic Chat Step

```yaml
steps:
  - name: summarize
    type: chat
    llm:
      provider: openai
      model: gpt-4o
    messages:
      - role: system
        content: "You are a concise summarizer."
      - role: user
        content: "Summarize: ${INPUT_TEXT}"
    output: SUMMARY
```

### DAG-Level LLM Defaults

Set `llm:` at the DAG level so all chat steps inherit the same provider/model:

```yaml
llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  temperature: 0.3
  max_tokens: 4096

steps:
  - name: step1
    type: chat
    messages:
      - role: user
        content: "Analyze this data: ${DATA}"
    output: ANALYSIS

  - name: step2
    type: chat
    messages:
      - role: user
        content: "Create a report from: ${ANALYSIS}"
    output: REPORT
```

### Providers

Supported providers and their default API key environment variables:

| Provider | API Key Env Var | Notes |
|----------|----------------|-------|
| `openai` | `OPENAI_API_KEY` | GPT-4o, o1, o3, GPT-5 |
| `anthropic` | `ANTHROPIC_API_KEY` | Claude 3.5, 4, 4.5 |
| `gemini` | `GOOGLE_API_KEY` | Gemini 2.5 |
| `openrouter` | `OPENROUTER_API_KEY` | Multi-provider gateway |
| `local` | — | Ollama, vLLM (use `base_url`) |

Override the API key env var with `api_key_name`:

```yaml
llm:
  provider: openai
  model: gpt-4o
  api_key_name: MY_CUSTOM_KEY
```

### Model Fallback

Specify an array of models for automatic fallback:

```yaml
llm:
  model:
    - provider: anthropic
      name: claude-sonnet-4-20250514
    - provider: openai
      name: gpt-4o
```

### Extended Thinking

Enable deeper reasoning for complex tasks:

```yaml
llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  thinking:
    enabled: true
    effort: high          # low, medium, high, xhigh
    budget_tokens: 10000  # explicit token budget (optional)
```

### Tool Calling

Chat steps can call other DAGs as tools:

```yaml
steps:
  - name: agent
    type: chat
    llm:
      provider: openai
      model: gpt-4o
      tools:
        - lookup-user       # another DAG that accepts params and returns output
        - send-notification
    messages:
      - role: user
        content: "Find user ${USER_ID} and send them a welcome message."
```

### Streaming

Streaming is enabled by default. Disable it for batch processing:

```yaml
llm:
  provider: openai
  model: gpt-4o
  stream: false
```

### Local Models (Ollama)

```yaml
llm:
  provider: local
  model: llama3
  base_url: http://localhost:11434/v1
```

## Agent Steps

Use `type: agent` for autonomous AI agents that can use tools (bash, read, patch, etc.) to complete tasks.

### Basic Agent Step

```yaml
steps:
  - name: analyze-logs
    type: agent
    messages:
      - role: user
        content: "Find errors in the log files and suggest fixes"
    agent:
      max_iterations: 20
    output: ANALYSIS
```

The agent model is resolved from Agent Settings (configured in the web UI), not from the `llm:` field.

### Agent Configuration

```yaml
steps:
  - name: fix-config
    type: agent
    messages:
      - role: user
        content: "Fix the YAML configuration issues"
    agent:
      model: my-model-id          # Override global default model
      max_iterations: 30          # Max tool call rounds (default: 50)
      safe_mode: true             # Require approval for bash commands (default: true)
      prompt: "Focus only on YAML syntax errors. Do not change logic."
      memory:
        enabled: true             # Load global + per-DAG memory
      skills:
        - dagu-containers         # Pre-load specific skills
```

### Tool Policy

Control which tools are available and restrict bash commands:

```yaml
steps:
  - name: read-only-analysis
    type: agent
    messages:
      - role: user
        content: "Analyze the codebase structure"
    agent:
      tools:
        enabled:
          - read
          - think
          - output
          # bash, patch excluded = read-only agent
        bash_policy:
          default_behavior: deny
          deny_behavior: block      # or "hitl" for human approval
          rules:
            - name: allow-ls
              pattern: "^ls "
              action: allow
            - name: allow-cat
              pattern: "^cat "
              action: allow
```

### Output Capture

Use the `output` tool to capture the agent's result for downstream steps:

```yaml
type: graph

steps:
  - name: research
    type: agent
    messages:
      - role: user
        content: "Research the topic and summarize findings"
    output: FINDINGS

  - name: report
    type: chat
    messages:
      - role: user
        content: "Write a report based on: ${FINDINGS}"
    depends:
      - research
```

## Chaining AI Steps

Combine chat and agent steps in a pipeline:

```yaml
type: graph

steps:
  - name: extract
    type: chat
    llm:
      provider: openai
      model: gpt-4o
    messages:
      - role: user
        content: "Extract key metrics from: ${RAW_DATA}"
    output: METRICS

  - name: analyze
    type: agent
    messages:
      - role: user
        content: "Analyze these metrics and create visualizations: ${METRICS}"
    agent:
      max_iterations: 30
    output: ANALYSIS
    depends:
      - extract

  - name: summarize
    type: chat
    messages:
      - role: user
        content: "Write an executive summary of: ${ANALYSIS}"
    output: SUMMARY
    depends:
      - analyze
```
