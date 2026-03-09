# Executor Types

## command / shell (default)

Shell command execution. Uses step `command:`, `script:`, or `shell:` fields.

```yaml
steps:
  - name: example
    command: echo "hello"
```

Aliases: (empty), `command`, `shell`

## docker

Run commands in Docker containers.

```yaml
steps:
  - name: build
    type: docker
    config:
      image: golang:1.22
      auto_remove: true
    command: go build ./...
```

Aliases: `docker`, `container`

Config fields:
- `image` — Docker image
- `container_name` — Container name
- `pull_policy` — Image pull policy
- `volumes` — Volume mounts
- `platform` — Target platform
- `auto_remove` — Remove container after exit
- `startup` — Startup mode: `keepalive`, `entrypoint`, `command`
- `wait_for` — Wait condition: `running`, `healthy`
- `log_pattern` — Log pattern to wait for

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

Execute same DAG multiple times in parallel.

```yaml
steps:
  - name: fan-out
    type: parallel
    call: process-item
    parallel:
      items:
        - item1
        - item2
        - item3
      max_concurrent: 5
```

## ssh

Remote command execution.

```yaml
steps:
  - name: remote
    type: ssh
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
    command: systemctl restart app
```

Config fields:
- `user` — SSH username
- `host` — Hostname
- `port` — Port (default 22)
- `key` — Path to private key
- `password` — Password
- `shell` — Remote shell
- `timeout` — Connection timeout (default 30s)
- `bastion` — Jump host config (same fields)

## sftp

File transfer over SSH.

```yaml
steps:
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

Config: same as ssh + `direction` (`upload`/`download`), `source`, `destination`

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
      body: '{"key": "value"}'
      json: true
```

Command format: `"METHOD URL"`. Config fields:
- `timeout` — Request timeout
- `headers` — HTTP headers
- `query` — Query parameters
- `body` — Request body
- `silent` — Suppress output
- `debug` — Enable debug logging
- `json` — Parse response as JSON
- `skip_tls_verify` — Skip TLS verification

## jq

JSON processing.

```yaml
steps:
  - name: transform
    type: jq
    command: ".items[] | {name: .name, count: .quantity}"
    script: '{"items": [{"name": "a", "quantity": 1}]}'
```

Query from `command:`, input JSON from `script:`. Config: `raw` (output raw strings)

## postgres

PostgreSQL queries.

```yaml
steps:
  - name: query
    type: postgres
    config:
      dsn: "postgres://user:pass@localhost:5432/db"
      output_format: json
    script: "SELECT * FROM users WHERE active = true"
```

Config fields:
- `dsn` — Connection string
- `params` — Query parameters
- `timeout` — Query timeout (default 60s)
- `transaction` — Run in transaction
- `isolation_level` — Transaction isolation
- `advisory_lock` — Advisory lock ID
- `output_format` — `jsonl`, `json`, `csv`
- `streaming` — Stream results
- `import` — Bulk import config

## sqlite

SQLite queries. Same config as postgres + `shared_memory`, `file_lock`.

```yaml
steps:
  - name: query
    type: sqlite
    config:
      dsn: "/path/to/db.sqlite"
    script: "SELECT * FROM logs"
```

## redis

Redis operations.

```yaml
steps:
  - name: cache
    type: redis
    config:
      url: "redis://localhost:6379"
      command: SET
      key: mykey
      value: myvalue
```

Config fields:
- `url` or `host`/`port`/`password`/`db` — Connection
- `command` — Redis command
- `key`, `value`, `field` — Operation parameters
- TLS options, sentinel/cluster mode, pipeline, Lua scripting, distributed locks, pub/sub, streams

## s3

S3 operations.

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

Commands: `upload`, `download`, `list`, `delete`. Config fields:
- `region`, `endpoint`, `bucket`, `key`
- `source`, `destination`
- `force_path_style` — Use path-style addressing
- Encryption, tagging, transfer options

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
      attachments:
        - /path/to/report.pdf
```

## archive

Archive operations.

```yaml
steps:
  - name: compress
    type: archive
    command: create
    config:
      source: /data/output
      destination: /data/output.tar.gz
      format: tar.gz
```

Commands: `create`, `extract`, `list`. Config fields:
- `source`, `destination`
- `format` — `zip`, `tar`, `tar.gz`
- `compression_level`
- `password`
- `include`/`exclude` — Glob patterns

## gha

Run GitHub Actions locally.

```yaml
steps:
  - name: checkout
    type: gha
    command: "actions/checkout@v4"
    config:
      runner: catthehacker/ubuntu:act-latest
```

Aliases: `gha`, `github_action`, `github-action`

Config fields:
- `runner` — Docker image for runner
- `auto_remove` — Remove container after
- `network` — Docker network
- `privileged` — Privileged mode

## chat

LLM chat step.

```yaml
steps:
  - name: summarize
    type: chat
    llm:
      provider: openai
      model: gpt-4
      system: "You are a helpful assistant."
    messages:
      - role: user
        content: "Summarize this: ${INPUT}"
```

Uses step `llm:` config and `messages:` for conversation.

## agent

AI agent loop with tools.

```yaml
steps:
  - name: research
    type: agent
    agent:
      model: claude-sonnet-4-20250514
      tools: [web_search, file_read]
      max_iterations: 10
      prompt: "Research and summarize ${TOPIC}"
```

Config: `model`, `tools`, `skills`, `soul`, `memory`, `prompt`, `max_iterations`, `safe_mode`

## router

Conditional routing based on expression value.

```yaml
steps:
  - name: route
    type: router
    value: ${STATUS}
    routes:
      "ok":
        - name: handle-ok
          command: echo "success"
      "re:err.*":
        - name: handle-error
          command: echo "error occurred"
```

Uses step `value:` (expression) and `routes:` (pattern→step mappings). Supports `re:regex` patterns.
