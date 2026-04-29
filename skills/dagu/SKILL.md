---
name: dagu
description: Writes, validates, and debugs DAG workflow definitions in YAML. Use when creating, editing, or troubleshooting DAGs.
---

# DAG Authoring

Load only the reference file that matches the task.

## Default Approach

- Prefer `type: graph` for new DAGs. It supports both sequential flow via `depends:` and parallel flow.
- Prefer `id` on every step. Omit `name` unless the display label must differ from the step ID.
- Prefer `dagu enqueue` over `dagu start` for agent-run workflows.
- Prefer `dagu schema ...` and `dagu validate ...` over guessing field names or shapes.
- Prefer `type: template` when generating text files, prompts, or artifacts instead of assembling them with shell `echo` or heredocs.
- Prefer `DAG_RUN_ARTIFACTS_DIR` for file outputs when possible, as it provides a preview in the UI and automatically cleans up when the DAG run gets deleted.
- Prefer `output:` for capturing stdout content into variables if the content is reasonably small and doesn't require complex parsing.
- Prefer temporary file in artifacts dir for larger outputs or when downstream steps need file paths.

## High-Signal Rules

- `output:` captures trimmed stdout content into a variable. `${step_id.stdout}` is a log file path, not stdout content.
- `env:` should use list-of-maps when values depend on earlier env vars.
- `params:` values arrive as strings. The `params:` field supports JSON schema-like types and validation, check for schema to see how to specify types and validation rules.
- Do not assume `bash` for `script:` steps. If a script depends on a specific interpreter, add a shebang such as `#!/bin/sh` or `#!/usr/bin/env bash` only after checking that shell exists on the target host or container. Otherwise keep the script portable or set `shell:` explicitly.
- `parallel:` requires `call:` to a sub-DAG.
- Sub-DAGs do not inherit parent env vars; pass what you need via `params:`.
- For arbitrary text inside shell steps, prefer `printenv VAR_NAME` or `type: template` over `${VAR}` interpolation.
- Use `dagu schema dag` to check the full list of available fields and their shapes.
- Use `dagu example` to see different DAG patterns and how to express them in YAML.

## Example of Params, template step, and artifacts

```yaml
params:
  type: object
  properties:
    name:
      type: string
      maxLength: 50
    age:
      type: integer
      minimum: 0
      maximum: 120
    favorite_color:
      type: string
  required: [name, age]

steps:
  - id: render
    type: template
    with:
      data:
        name: ${name}
        age: ${age}
        favorite_color: ${favorite_color}
    script: |
      Hello, {{ .name }}!
      You are {{ .age }} years old.
      {{- if .favorite_color }}
      Your favorite color is {{ .favorite_color }}.
      {{- end }}
    stdout: ${DAG_RUN_ARTIFACTS_DIR}/greeting.txt
```

## Reference Guide

Load only the file you need:

- `references/steptypes.md` when choosing a step type or checking executor-specific caveats such as `dag`, `parallel`, `jq`, or `template`
- `references/cli.md` when you need command flags or lookup commands such as `dagu schema`, `dagu config`, or `dagu history`
- `references/env.md` when execution environment variables, `DAGU_*` config vars, or `params:`/`env:` resolution order matters
- `references/codingagent.md` only when the DAG itself runs AI coding agents as steps
