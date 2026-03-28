# Executor Types

## command / shell (default)

Shell command execution. Uses step `command:`, `script:`, or `shell:` fields.

```yaml
steps:
  - name: example
    command: echo "hello"

  - name: multi-line
    script: |
      echo "step 1"
      echo "step 2"

  - name: custom-shell
    shell: /bin/bash
    script: |
      set -euo pipefail
      echo "running in bash"
```

Aliases: (empty), `command`, `shell`

Step-level fields:
- `command` — Command string to execute
- `args` — Arguments for the command
- `script` — Multi-line shell script content
- `shell` — Shell interpreter (e.g., `/bin/bash`)

## docker

Run commands in Docker containers.

```yaml
steps:
  - name: build
    type: docker
    config:
      image: golang:1.23
      pull: always
      auto_remove: true
      working_dir: /app
      volumes:
        - /local/src:/app
    command: go build ./...
```

Aliases: `docker`, `container`

Config fields:
- `image` — Docker image (required unless `container_name` is set)
- `container_name` — Name/ID of existing container for exec mode
- `pull` — Image pull policy: `always`, `never`, `missing` (default)
- `auto_remove` — Remove container after exit
- `working_dir` — Working directory inside container
- `volumes` — Volume mounts (list of `host:container` strings)
- `shell` — Shell wrapper for step commands (e.g., `["/bin/bash", "-c"]`)

## dag

Execute another DAG as a sub-step.

```yaml
steps:
  - name: child
    type: dag
    call: child-workflow
    params:
      input: /data/file.csv
```

Aliases: `dag`, `subworkflow`

Uses step `call:` and `params:` fields. Sub-DAGs do not inherit parent env vars.

## parallel

Execute same DAG multiple times in parallel. Requires `call:` field.

```yaml
steps:
  # Simple list of items
  - name: fan-out
    call: process-item
    parallel:
      - item1
      - item2
      - item3

  # Object form with concurrency control
  - name: fan-out-limited
    call: process-item
    parallel:
      items:
        - item1
        - item2
        - item3
      max_concurrent: 5

  # Items with key-value parameters
  - name: fan-out-params
    call: process-item
    parallel:
      items:
        - SOURCE: s3://customers
        - SOURCE: s3://products

  # Variable reference (JSON array)
  - name: fan-out-dynamic
    call: process-item
    parallel: ${ITEMS}
```

Config fields:
- `items` — Array of items to process (strings or key-value param maps)
- `max_concurrent` — Max parallel executions (default 10)

Each parallel invocation receives the current item as the `ITEM` variable.

## ssh / sftp

Remote command execution and file transfer over SSH.

```yaml
steps:
  - name: remote
    type: ssh
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
      timeout: 60s
    command: systemctl restart app

  - name: upload
    type: sftp
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
      direction: upload
      source: /local/file.tar.gz
      destination: /remote/file.tar.gz
```

Shared SSH config fields: `user`, `host`, `port` (default 22), `key`, `password`, `timeout` (default 30s), `strict_host_key` (default true), `known_host_file`, `shell`, `shell_args`, `bastion` (jump host with `host`, `port`, `user`, `key`, `password`).

SFTP additional fields: `direction` (`upload` or `download`), `source`, `destination`.

## http

HTTP requests.

```yaml
steps:
  - name: api-call
    type: http
    command: "POST https://api.example.com/data"
    config:
      headers:
        Authorization: "Bearer ${TOKEN}"
        Content-Type: application/json
      body: '{"key": "value"}'
      json: true
      timeout: 30
```

Command format: `"METHOD URL"`. Config fields: `timeout` (seconds), `headers` (map), `query` (map), `body` (string), `silent`, `debug`, `json`, `skip_tls_verify`.

## jq

JSON processing.

```yaml
steps:
  - name: transform
    type: jq
    command: ".items[] | {name: .name, count: .quantity}"
    script: '{"items": [{"name": "a", "quantity": 1}]}'
```

Query from `command:`, input JSON from `script:`. Config fields:
- `raw` — Output raw strings without JSON encoding (like `jq -r`)

## template

Render text using Go `text/template`.

```yaml
steps:
  - id: render
    type: template
    config:
      data:
        name: Alice
    script: |
      Hello, {{ .name }}!
    output: RESULT
```

Behavior:
- `script` is required and is rendered as a template, not executed as a shell script
- Template data comes from `config.data` and is accessed as `{{ .key }}`
- Supports normal Go template control flow plus a safe subset of slim-sprig functions
- Missing keys fail the step
- If `config.output` is set, the rendered result is written to that file instead of stdout
- Relative `config.output` paths are resolved from the step working directory

Config fields:
- `data` — Object exposed to the template as `.`
- `output` — File path for rendered output; if omitted, rendered text is written to stdout

Important: step `output:` and `config.output` are different. Step `output:` captures stdout into a Dagu variable. `config.output` writes the rendered result directly to a file.

Use `template` when you need to generate text files such as Markdown, config files, SQL, JSON, or prompts. It is usually safer and simpler than building files with `echo`, heredocs, or shell string interpolation.

## sql (postgres / sqlite)

SQL database queries. Use `type: postgres` or `type: sqlite`.

```yaml
steps:
  - name: query
    type: postgres
    config:
      dsn: "postgres://user:pass@localhost:5432/db"
      output_format: json
      timeout: 120
      transaction: true
    script: "SELECT * FROM users WHERE active = true"
```

SQL from `command:` (single statement) or `script:` (multiple statements, also supports `file:///path/to/file.sql`).

Config fields: `dsn` (required), `params` (map or array), `timeout` (seconds), `transaction`, `isolation_level` (`default`, `read_committed`, `repeatable_read`, `serializable`), `output_format` (`jsonl`, `json`, `csv`), `headers`, `null_string`, `max_rows`, `streaming`, `output_file`, `import` (bulk import config with `input_file`, `table`, `format`, `batch_size`, etc.).

SQLite-specific: `shared_memory` (shared cache for `:memory:` DBs across steps), `file_lock`. PostgreSQL-specific: `advisory_lock`.

## redis

Redis operations.

```yaml
steps:
  - name: cache-set
    type: redis
    config:
      url: "redis://localhost:6379"
      command: SET
      key: mykey
      value: myvalue
      ttl: 3600
```

Connection: `url` (e.g., `redis://user:pass@host:port/db`), or `host`/`port`/`password`/`username`/`db`. TLS: `tls`, `tls_cert`, `tls_key`, `tls_ca`, `tls_skip_verify`. Modes: `mode` (`standalone`, `sentinel`, `cluster`), `timeout`, `max_retries`.

Command fields: `command` (e.g., `SET`, `GET`, `HSET`, `LPUSH`), `key`/`keys`, `value`/`values`, `field`/`fields`, `ttl`, `nx`, `xx`. Output: `output_format` (`json`, `jsonl`, `raw`, `csv`).

## s3

S3 operations. Commands: `upload`, `download`, `list`, `delete`.

```yaml
steps:
  - name: upload
    type: s3
    command: upload
    config:
      region: us-east-1
      bucket: my-bucket
      key: data/output.csv
      source: /local/output.csv
```

Connection: `region`, `endpoint`, `access_key_id`, `secret_access_key`, `session_token`, `profile`, `force_path_style`.

Object fields: `bucket` (required), `key`, `source` (for upload), `destination` (for download), `content_type`, `storage_class`, `metadata` (map), `tags` (map). List: `prefix`, `delimiter`, `max_keys`, `recursive`, `output_format`.

## mail

Send email.

```yaml
steps:
  - name: notify
    type: mail
    config:
      from: noreply@example.com
      to: team@example.com
      subject: "Build Complete"
      message: "The build finished successfully."
```

Config fields:
- `from` — Sender email address
- `to` — Recipient(s) (string or array of strings)
- `subject` — Email subject line
- `message` — Email body content

Note: SMTP server is configured via Dagu's global settings, not per-step.

## archive

Archive operations. Commands: `create`, `extract`, `list`.

```yaml
steps:
  - name: compress
    type: archive
    command: create
    config:
      source: /data/output
      destination: /data/output.tar.gz
      format: tar.gz
      exclude:
        - "*.tmp"
```

Config fields: `source` (required), `destination` (required for create), `format` (`zip`, `tar`, `tar.gz`, etc.; inferred from filename if omitted), `compression_level`, `password` (extract/list only), `overwrite`, `strip_components`, `include`/`exclude` (glob patterns).

## gha

Run GitHub Actions locally. Aliases: `gha`, `github_action`, `github-action`.

```yaml
steps:
  - name: checkout
    type: gha
    command: "actions/checkout@v4"
    config:
      runner: catthehacker/ubuntu:act-latest
      auto_remove: true
```

Config fields: `runner` (Docker image), `auto_remove`, `network`, `privileged`, `github_instance`, `reuse_containers`, `force_rebuild`. Step-level: `command` (action reference), `params` (action inputs).

## chat

LLM chat step.

```yaml
steps:
  - name: summarize
    type: chat
    llm:
      provider: anthropic
      model: claude-sonnet-4-20250514
      system: "You are a helpful assistant."
      temperature: 0.7
      max_tokens: 2000
      stream: true
    messages:
      - role: user
        content: "Summarize this: ${INPUT}"
```

Uses step `llm:` config and `messages:` for conversation. Message roles: `system`, `user`, `assistant`, `tool`.

LLM config fields: `provider` (`openai`, `anthropic`, `gemini`, `openrouter`, `local`), `model`, `system`, `temperature` (0.0-2.0), `max_tokens`, `top_p`, `base_url`, `api_key_name`, `stream` (default true), `tools` (list of DAG names as callable tools), `max_tool_iterations` (default 10).

## agent

AI agent loop with tools.

```yaml
steps:
  - name: research
    type: agent
    agent:
      model: claude-sonnet-4-20250514
      tools:
        enabled:
          - web_search
          - bash
      skills:
        - my-skill-id
      prompt: "Research and summarize ${TOPIC}"
      max_iterations: 50
      safe_mode: true
    messages:
      - role: user
        content: "Begin research on ${TOPIC}"
```

Agent config fields (under `agent:`): `model`, `tools` (with `enabled` list and optional `bash_policy`), `skills` (skill IDs), `soul` (soul ID), `memory` (`enabled` bool), `prompt` (appended to system prompt), `max_iterations` (default 50), `safe_mode` (enable command approval, default true), `web_search`. Also accepts `messages:` at step level.

## router

Conditional routing based on expression value. Routes reference existing step names — they do not define inline steps.

```yaml
steps:
  - name: check-status
    command: "curl -s -o /dev/null -w '%{http_code}' https://example.com"
    output: STATUS

  - name: route
    type: router
    value: ${STATUS}
    depends: check-status
    routes:
      "200":
        - handle-ok
      "re:5\\d{2}":
        - handle-error
        - send-alert

  - name: handle-ok
    command: echo "success"

  - name: handle-error
    command: echo "server error occurred"

  - name: send-alert
    command: echo "alerting on-call"
```

Step-level fields:
- `value` — Expression to evaluate (required)
- `routes` — Map of pattern to list of target step names (required)

Pattern matching:
- Exact match: `"200"` matches the literal value `200`
- Regex match: `"re:5\\d{2}"` matches `500`, `502`, etc.
- Catch-all: `"re:.*"` matches anything (sorted last automatically)

Routing rules:
- Routes are evaluated in priority order: exact matches first, then regex, then catch-all
- Each target step can only be targeted by one route pattern
- Multiple targets per route execute in parallel
- Steps not targeted by any matching route are skipped
