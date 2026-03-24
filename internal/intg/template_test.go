// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateExecutor(t *testing.T) {
	t.Parallel()

	t.Run("StdoutOnly", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      data:
        greeting: hello
    script: |
      {{ .greeting }}, world!
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": test.Contains("hello, world!"),
		})
	})

	t.Run("FileOnly", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "report.md")

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      output: "`+outFile+`"
      data:
        title: Test Report
    script: |
      # {{ .title }}
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)

		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "# Test Report")
	})

	t.Run("RelativeOutputPath", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    working_dir: "`+tmpDir+`"
    config:
      output: "subdir/output.txt"
      data:
        msg: relative
    script: "{{ .msg }}"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)

		content, err := os.ReadFile(filepath.Join(tmpDir, "subdir", "output.txt"))
		require.NoError(t, err)
		assert.Equal(t, "relative", string(content))
	})

	t.Run("DataFromPriorStep", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `
type: graph
steps:
  - id: producer
    command: 'echo -n "Alice"'
    output: NAME

  - id: render
    depends:
      - producer
    type: template
    config:
      data:
        name: ${NAME}
    script: "Hello, {{ .name }}!"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "Hello, Alice!",
		})
	})

	t.Run("LiteralDollarPreservation", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      data:
        name: test
    script: |
      export FOO=${BAR}
      echo "{{ .name }}"
      value=`+"`command`"+`
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": []test.Contains{
				test.Contains("${BAR}"),
				test.Contains("`command`"),
			},
		})
	})

	t.Run("MissingKeyError", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      data:
        name: test
    script: "{{ .undefined_key }}"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunCheckErr(t, "execution error")

		dag.AssertLatestStatus(t, core.Failed)
	})

	t.Run("ComplexTemplate", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      data:
        title: Domain Report
        domains: "example.com,test.org,demo.net"
    script: |
      # {{ .title }}
      {{ $items := .domains | split "," }}
      Total: {{ $items | count }}
      {{ range $i, $d := $items }}
      {{ $i | add 1 }}. {{ $d | upper }}
      {{ end }}
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": []test.Contains{
				test.Contains("# Domain Report"),
				test.Contains("Total: 3"),
				test.Contains("EXAMPLE.COM"),
				test.Contains("TEST.ORG"),
				test.Contains("DEMO.NET"),
			},
		})
	})

	t.Run("ConditionalAndEmpty", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      data:
        items: ""
    script: |
      {{ if .items | empty }}No items found.{{ else }}Has items.{{ end }}
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": test.Contains("No items found."),
		})
	})

	t.Run("DefaultFunction", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: render
    type: template
    config:
      data:
        name: ""
        title: Admin
    script: '{{ .name | default "Anonymous" }} ({{ .title | default "User" }})'
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"RESULT": "Anonymous (Admin)",
		})
	})
}
