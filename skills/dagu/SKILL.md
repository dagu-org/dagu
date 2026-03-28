---
name: dagu
description: Writes, validates, and debugs Dagu DAG workflow definitions in YAML. Use when creating, editing, or troubleshooting Dagu .yaml DAG files. Do not use for general YAML editing.
---
# Dagu DAG Authoring

Use this skill for Dagu DAG YAML work. Keep the main skill lean and load only the reference file that matches the task.

## Default Approach

- Preserve existing DAG style when editing unless there is a clear reason to normalize it.
- Prefer `type: graph` for new DAGs. It supports both sequential flow via `depends:` and parallel flow.
- Prefer `id` on every step. Omit `name` unless the display label must differ from the step ID.
- Prefer `dagu enqueue` over `dagu start` for agent-run workflows.
- Prefer `dagu schema ...` and `dagu validate ...` over guessing field names or shapes.
- Prefer `type: template` when generating text files, prompts, or config artifacts instead of assembling them with shell `echo` or heredocs.

## High-Signal Rules

- `output:` captures trimmed stdout content into a variable. `${step_id.stdout}` is a log file path, not stdout content.
- `env:` should use list-of-maps when values depend on earlier env vars.
- `params:` values arrive as strings.
- Do not assume `bash` for `script:` steps. If a script depends on a specific interpreter, add a shebang such as `#!/bin/sh` or `#!/usr/bin/env bash` only after checking that shell exists on the target host or container. Otherwise keep the script portable or set `shell:` explicitly.
- `parallel:` requires `call:` to a sub-DAG.
- Sub-DAGs do not inherit parent env vars; pass what you need via `params:`.
- For arbitrary text inside shell steps, prefer `printenv VAR_NAME` or `type: template` over `${VAR}` interpolation.

## Minimal Example

```yaml
type: graph
steps:
  - id: fetch
    command: echo '{"name":"Alice"}'
    output: USER

  - id: render
    depends: [fetch]
    type: template
    config:
      data:
        name: ${USER.name}
      output: report.txt
    script: |
      Hello, {{ .name }}!
```

## Reference Guide

Load only the file you need:

- `references/executors.md` when choosing a step type or checking executor-specific caveats such as `dag`, `parallel`, `jq`, or `template`
- `references/schema.md` when you need exact field names or runtime behavior for `output:`, retries, preconditions, scheduling, or repeat limits
- `references/cli.md` when you need command flags or lookup commands such as `dagu schema`, `dagu config`, or `dagu history`
- `references/env.md` when execution environment variables, `DAGU_*` config vars, or `params:`/`env:` resolution order matters
- `references/codingagent.md` only when the DAG itself runs AI coding agents as steps
