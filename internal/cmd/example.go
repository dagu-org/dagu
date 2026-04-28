// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type exampleEntry struct {
	ID          int
	Name        string
	Description string
	Content     string
}

var examples = []exampleEntry{
	{
		ID:          1,
		Name:        "parallel-steps",
		Description: "Run steps in parallel using depends",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
steps:
  - name: setup
    command: echo "preparing data"
  - name: task-a
    command: echo "processing batch A"
    depends: [setup]
  - name: task-b
    command: echo "processing batch B"
    depends: [setup]
  - name: task-c
    command: echo "processing batch C"
    depends: [setup]
  - name: aggregate
    command: echo "all tasks finished"
    depends: [task-a, task-b, task-c]
`,
	},
	{
		ID:          2,
		Name:        "output-passing",
		Description: "Capture step output and pass between steps",
		Content: `type: graph
steps:
  - name: get-version
    command: echo "2.5.0"
    output: VERSION
  - name: get-metadata
    command: echo '{"build":"abc123","env":"staging"}'
    output:
      name: BUILD_ID
      key: build
    depends: [get-version]
  - name: deploy
    command: echo "deploying v${VERSION} build ${BUILD_ID}"
    depends: [get-version, get-metadata]
`,
	},
	{
		ID:          3,
		Name:        "schedule-params-env",
		Description: "Schedule a DAG with params and env vars",
		Content: `type: graph
schedule: "0 2 * * *"
catchup_window: "12h"
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
params:
  - name: ENV
    type: string
    enum: [DEV, STG, PROD]
    description: Target environment for the scheduled batch run
    required: true
    default: STG
  - name: BATCH_SIZE
    type: integer
    minimum: 1
    maximum: 1000
    description: Number of records processed per batch
    required: true
    default: 100
env:
  - LOG_LEVEL: "info"
  - TIMESTAMP: "` + "`date +%Y%m%d`" + `"
steps:
  - name: extract
    command: echo "extracting ${BATCH_SIZE} records in ${ENV}"
  - name: transform
    command: echo "transforming with LOG_LEVEL=${LOG_LEVEL}"
    depends: [extract]
  - name: load
    command: echo "loading batch from ${TIMESTAMP}"
    depends: [transform]
`,
	},
	{
		ID:          4,
		Name:        "defaults-and-retry",
		Description: "Set step defaults with retry and continue_on",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 3
    interval_sec: 5
  continue_on: failed
steps:
  - name: fetch-data
    command: "curl -sf https://httpbin.org/status/200 || exit 1"
  - name: process
    command: echo "processing data"
    depends: [fetch-data]
  - name: cleanup
    command: echo "done"
    retry_policy:
      limit: 1
      interval_sec: 1
    depends: [process]
`,
	},
	{
		ID:          5,
		Name:        "preconditions",
		Description: "Guard steps with preconditions",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
params:
  - name: ENV
    type: string
    enum: [DEV, STG, PROD]
    description: Deployment environment; only PROD satisfies the gate
    required: true
    default: STG
steps:
  - name: check-env
    command: echo "verifying environment"
  - name: deploy
    command: echo "deploying application"
    preconditions:
      - condition: "${ENV}"
        expected: "PROD"
    depends: [check-env]
  - name: notify
    command: echo "deployment complete"
    depends: [deploy]
`,
	},
	{
		ID:          6,
		Name:        "lifecycle-hooks",
		Description: "Use handler_on for init, success, failure, exit",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
handler_on:
  init:
    command: echo "workflow starting"
  success:
    command: echo "all steps succeeded"
  failure:
    command: echo "a step failed"
  exit:
    command: echo "cleanup complete"
steps:
  - name: step-1
    command: echo "running step 1"
  - name: step-2
    command: echo "running step 2"
    depends: [step-1]
`,
	},
	{
		ID:          7,
		Name:        "http-requests",
		Description: "Make HTTP requests and use responses",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
steps:
  - name: get-todo
    type: http
    command: "GET https://jsonplaceholder.typicode.com/todos/1"
    output: TODO
  - name: show-result
    command: echo "Received - ${TODO}"
    depends: [get-todo]
`,
	},
	{
		ID:          8,
		Name:        "docker-container",
		Description: "Run steps inside a Docker container",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
container:
  image: python:3.13-slim
  volumes:
    - /tmp/dagu-example:/work
steps:
  - name: write-data
    command: >-
      python -c "with open('/work/data.txt', 'w') as f: f.write('Hello from Dagu!')"
  - name: process
    command: >-
      python -c "with open('/work/data.txt') as f: print(f.read().upper())"
    depends: [write-data]
`,
	},
	{
		ID:          9,
		Name:        "sub-dag",
		Description: "Call another DAG as a sub-workflow",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
steps:
  - name: prepare
    command: echo "starting main workflow"
  - name: run-etl
    call: etl-job
    params: "SOURCE=/data/input.csv TARGET=/data/output.csv"
    depends: [prepare]
  - name: done
    command: echo "pipeline complete"
    depends: [run-etl]
---
name: etl-job
params:
  - name: SOURCE
    type: string
    description: Input dataset or file path received from the parent DAG
    required: true
    default: /data/default-input.csv
  - name: TARGET
    type: string
    description: Output dataset or file path produced by the sub-DAG
    required: true
    default: /data/default-output.csv
type: graph
steps:
  - name: extract
    command: echo "extracting from ${SOURCE}"
  - name: load
    command: echo "loading into ${TARGET}"
    depends: [extract]
`,
	},
	{
		ID:          10,
		Name:        "conditional-routing",
		Description: "Route execution based on step output",
		Content: `type: graph
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
steps:
  - name: check-status
    command: "echo success"
    output: STATUS
  - name: route
    type: router
    value: ${STATUS}
    routes:
      success: [on-success]
      "re:.*": [on-failure]
    depends: [check-status]
  - name: on-success
    command: echo "status was success"
  - name: on-failure
    command: echo "status was something else"
`,
	},
	{
		ID:          11,
		Name:        "approval-gate",
		Description: "Review, push back with rewind_to, then continue to deploy",
		Content: `type: graph
artifacts:
  enabled: true
steps:
  - id: build
    command: echo "v1.2.3"
    output: VERSION
  - id: prepare_release_notes
    depends: [build]
    type: template
    with:
      data:
        version: ${VERSION}
        feedback: ${FEEDBACK}
        deploy_window: ${DEPLOY_WINDOW}
    script: |
      Release {{ .version }}
      Summary: {{ .feedback | default "Initial release notes draft" }}
      Deployment window: {{ .deploy_window | default "Pending approval input" }}
    output: RELEASE_NOTES
  - id: review_release
    depends: [prepare_release_notes]
    type: template
    with:
      output: ${DAG_RUN_ARTIFACTS_DIR}/release-notes.md
      data:
        version: ${VERSION}
        release_notes: ${RELEASE_NOTES}
        deploy_window: ${DEPLOY_WINDOW}
    script: |
      # Release {{ .version }}
      
      ## Summary
      
      {{ .release_notes }}
      
      ## Deployment Window
      
      {{ .deploy_window | default "Pending approval input" }}
    approval:
      prompt: "Review the release-notes.md artifact. Push back with FEEDBACK to regenerate it, or approve with DEPLOY_WINDOW to continue."
      input: [FEEDBACK, DEPLOY_WINDOW]
      required: [DEPLOY_WINDOW]
      rewind_to: prepare_release_notes
  - id: deploy
    depends: [review_release]
    command: echo "deploying ${VERSION} during ${DEPLOY_WINDOW}"
`,
	},
	{
		ID:          12,
		Name:        "agent-step",
		Description: "Build the agent prompt with a template and write a report artifact",
		Content: `type: graph
artifacts:
  enabled: true
defaults:
  retry_policy:
    limit: 2
    interval_sec: 5
steps:
  - id: gather_logs
    command: 'echo "error: connection timeout at 10:23 AM"'
    output: ERROR_LOG
  - id: build_prompt
    type: template
    with:
      data:
        error_log: ${ERROR_LOG}
    script: |
      Analyze this incident log and suggest a fix:
      
      {{ .error_log }}
    output: ANALYSIS_PROMPT
    depends: [gather_logs]
  - id: analyze
    type: agent
    agent:
      prompt: "You are a concise incident analyst."
      max_iterations: 10
    messages:
      - role: user
        content: ${ANALYSIS_PROMPT}
    output: ANALYSIS
    depends: [build_prompt]
  - id: report
    type: template
    with:
      output: ${DAG_RUN_ARTIFACTS_DIR}/report.md
      data:
        error_log: ${ERROR_LOG}
        analysis: ${ANALYSIS}
    script: |
      # Incident Report
      
      ## Error Log
      
      {{ .error_log }}
      
      ## Analysis
      
      {{ .analysis }}
    depends: [analyze]
`,
	},
}

// ExampleCount returns the number of available examples.
func ExampleCount() int { return len(examples) }

// Example creates the 'example' CLI command that displays example DAG definitions.
func Example() *cobra.Command {
	return &cobra.Command{
		Use:   "example [id]",
		Short: "Show example DAG definitions",
		Long: `Display example DAG definitions to help you get started.

Run without arguments to list all available examples.
Use a numeric ID to show a specific example.`,
		Example: `  dagu example      List all available examples
  dagu example 1    Show the parallel-steps example
  dagu example 7    Show the http-requests example`,
		ValidArgs: func() []string {
			args := make([]string, len(examples))
			for i, e := range examples {
				args[i] = strconv.Itoa(e.ID)
			}
			return args
		}(),
		Args: cobra.MaximumNArgs(1),
		RunE: runExample,
	}
}

func runExample(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if len(args) == 0 {
		return listExamples(cmd)
	}

	if args[0] == "help" {
		return cmd.Help()
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid example ID %q: must be a number between 1 and %d", args[0], len(examples))
	}

	if id < 1 || id > len(examples) {
		return fmt.Errorf("invalid example ID %q: must be between 1 and %d", args[0], len(examples))
	}

	e := examples[id-1]
	_, _ = fmt.Fprintf(w, "# Example %d: %s\n", e.ID, titleCase(e.Name))
	_, _ = fmt.Fprintf(w, "# %s\n\n", e.Description)
	_, _ = fmt.Fprint(w, e.Content)

	return nil
}

func listExamples(cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	_, _ = fmt.Fprintln(w, "Available DAG examples:")
	_, _ = fmt.Fprintln(w)

	for _, e := range examples {
		_, _ = fmt.Fprintf(w, "  %-4d %-24s %s\n", e.ID, e.Name, e.Description)
	}

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Usage: dagu example <id>")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, `Tip: Use "dagu schema dag" to explore all DAG fields and options.`)

	return nil
}

func titleCase(s string) string {
	words := strings.Split(s, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
