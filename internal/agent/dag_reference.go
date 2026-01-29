package agent

import (
	"encoding/json"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

// DagReferenceInput is the input schema for the DAG reference tool.
type DagReferenceInput struct {
	Section string `json:"section"`
}

// NewDagReferenceTool creates a tool that returns DAG YAML reference documentation.
func NewDagReferenceTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_dag_reference",
				Description: "Get DAG YAML reference documentation. ALWAYS call this before creating or editing DAG files to ensure correct syntax.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"section": map[string]any{
							"type":        "string",
							"description": "Section to retrieve: overview, steps, executors, containers, subdags, examples",
							"enum":        []string{"overview", "steps", "executors", "containers", "subdags", "examples"},
						},
					},
					"required": []string{"section"},
				},
			},
		},
		Run: dagReferenceRun,
	}
}

var dagReferenceSections = map[string]string{
	"overview": `# DAG YAML Overview

## Minimal DAG
steps:
  - command: echo hello

## Complete Structure (all fields optional except steps)
name: string                    # defaults to filename
schedule: "cron-expr"           # or array, or {start, stop, restart}
env:
  - KEY: value
params: "KEY=value KEY2=value2" # or array of maps
tags:
  - name: value
steps: [...]                    # required
handlerOn:                      # lifecycle hooks
  success: {command: ...}
  failure: {command: ...}
  exit: {command: ...}

## Execution Types
type: chain  # (default) sequential - each step auto-depends on previous
type: graph  # parallel - must use explicit 'depends:', steps without it run immediately

## Variables
${PARAM}                       # parameter reference
${ENV_VAR:-default}            # env var with default
` + "`command`" + `                      # command substitution
${OUTPUT.json.path}            # JSON path from output

## Built-in Variables
DAG_NAME, DAG_RUN_ID, DAG_RUN_LOG_FILE, DAG_RUN_STEP_NAME
`,

	"steps": `# Step Fields

- name: step-name              # optional, auto-generated
- command: string|array        # shell command
- script: |                    # multi-line script (alternative to command)
    #!/bin/bash
    echo hello
- depends: [step1, step2]      # dependencies (for type: graph)
- output: VAR_NAME             # capture stdout to variable
- env: [{KEY: value}]          # step-specific env vars
- workingDir: /path            # working directory
- preconditions:               # skip if condition fails
    - condition: "${VAR}"
      expected: "value"
- continueOn:                  # continue on failure
    failure: true
- retryPolicy:                 # retry on failure
    limit: 3
    intervalSec: 10

## Example
steps:
  - name: build
    command: go build -o app
    workingDir: /src
  - name: test
    command: go test ./...
    depends: [build]
    retryPolicy:
      limit: 3
      intervalSec: 5
`,

	"containers": `# Docker/Container Execution

Two modes: Image mode (create container) or Exec mode (use existing)

## Image Mode
container:
  image: python:3.11          # required
  volumes: ["./src:/app"]
  env: [KEY=value]
  workingDir: /app
  user: "1000:1000"
  platform: linux/amd64
  ports: ["8080:8080"]
  network: bridge
  shell: ["/bin/bash", "-c"]  # wrap commands with shell
  # DAG-level only:
  startup: keepalive          # keepalive | entrypoint | command
  waitFor: running            # running | healthy
  keepContainer: true

## Exec Mode (use existing container)
container: my-container       # string form
container:
  exec: my-container          # object form
  user: root
  workingDir: /app

## Example
steps:
  - name: build
    container:
      image: golang:1.21
      volumes: ["./:/app"]
      workingDir: /app
    command: go build -o app
`,

	"executors": `# Step Types (Executors)

## http - API calls
- type: http
  command: GET https://api.example.com
  config: {headers: {...}, timeout: 30, body: "..."}

## ssh - Remote execution
- type: ssh
  command: ls -la
  config: {user: ubuntu, host: server.com, key: ~/.ssh/id_rsa}

## jq - JSON processing
- type: jq
  command: '.items[] | .name'
  script: '{"items": [...]}'

## mail - Send email
- type: mail
  config: {from: a@b.com, to: [c@d.com], subject: "...", message: "..."}

## s3 - S3 operations
- type: s3
  command: upload  # or download, list, delete
  config: {bucket: my-bucket, key: path/to/obj, source: /local/file}

## postgres/sqlite - SQL database
- type: postgres
  command: SELECT * FROM users WHERE id = $1
  config: {dsn: postgres://..., params: [123]}

## redis - Redis operations
- type: redis
  config: {url: redis://localhost:6379, command: GET, key: mykey}

## archive - Archive operations
- type: archive
  command: extract  # or create, list
  config: {source: /path/to/file.zip, destination: /output}

## hitl - Human-in-the-loop approval
- type: hitl
  config: {prompt: "Approve?", input: [reason], required: [reason]}

## chat - LLM conversation
- type: chat
  config: {provider: openai, model: gpt-4}
  messages: [{role: user, content: "..."}]

## gha - GitHub Actions (experimental)
- type: gha
  command: actions/checkout@v4
  params: {fetch-depth: 0}
`,

	"subdags": `# Sub-DAGs

## Call by name
- call: sub-workflow
  params: "INPUT=${value}"

## Call by file path
- call: workflows/external.yaml

## Embedded Sub-DAGs (same file)
Use '---' to define multiple DAGs. Root DAG: name optional. Sub-DAGs: name required.

steps:
  - call: processor
---
name: processor
steps:
  - command: echo "processing"

## Parallel Items
- call: worker
  parallel:
    items: [a, b, c]
    maxConcurrent: 2
  params: "ITEM=${ITEM}"
`,

	"examples": `# Common Examples

## Simple sequential DAG
steps:
  - name: step1
    command: echo "first"
  - name: step2
    command: echo "second"

## Parallel execution with dependencies
type: graph
steps:
  - name: fetch-a
    command: curl https://api.a.com
  - name: fetch-b
    command: curl https://api.b.com
  - name: combine
    command: ./combine.sh
    depends: [fetch-a, fetch-b]

## With parameters
params: "ENV TARGET"
steps:
  - name: deploy
    command: ./deploy.sh $ENV $TARGET

## Scheduled with retry
schedule: "0 8 * * *"
steps:
  - name: backup
    command: ./backup.sh
    retryPolicy:
      limit: 3
      intervalSec: 60

## Docker container
steps:
  - name: test
    container:
      image: python:3.11
      volumes: ["./:/app"]
      workingDir: /app
    command: pytest

## With lifecycle hooks
handlerOn:
  success:
    command: notify.sh "DAG succeeded"
  failure:
    command: notify.sh "DAG failed"
steps:
  - command: ./process.sh
`,
}

func dagReferenceRun(_ ToolContext, input json.RawMessage) ToolOut {
	var params DagReferenceInput
	if err := json.Unmarshal(input, &params); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	content, ok := dagReferenceSections[strings.ToLower(params.Section)]
	if !ok {
		return toolError("Unknown section: %s. Available: overview, steps, executors, containers, subdags, examples", params.Section)
	}

	return ToolOut{Content: content}
}
