// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec_test

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/spec"
	"github.com/stretchr/testify/require"
)

func TestStepOutputsDeclarationBuilds(t *testing.T) {
	t.Parallel()

	dag, err := spec.LoadYAML(context.Background(), []byte(`
name: test
steps:
  - id: build
    run: echo ok
    outputs:
      - name: image_tag
      - name: metadata
        type: json
`), spec.WithoutEval())
	require.NoError(t, err)
	require.Len(t, dag.Steps, 1)
	require.Equal(t, []ir.StepOutputDeclaration{
		{Name: "image_tag", Type: ir.StepDeclaredOutputTypeString},
		{Name: "metadata", Type: ir.StepDeclaredOutputTypeJSON},
	}, dag.Steps[0].Outputs)
}

func TestStepOutputsDeclarationValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		yaml    string
		message string
	}{
		{
			name: "null",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs: null
`,
			message: "outputs must be a non-empty sequence",
		},
		{
			name: "empty",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs: []
`,
			message: "outputs must be a non-empty sequence",
		},
		{
			name: "missing id",
			yaml: `
name: test
steps:
  - name: build
    run: echo ok
    outputs:
      - name: image_tag
`,
			message: "a step with outputs must define id",
		},
		{
			name: "missing name",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs:
      - type: string
`,
			message: "name is required",
		},
		{
			name: "invalid name",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs:
      - name: 1invalid
`,
			message: "name must match",
		},
		{
			name: "duplicate name",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs:
      - name: image_tag
      - name: image_tag
`,
			message: "duplicate output name",
		},
		{
			name: "unknown field",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs:
      - name: image_tag
        value: latest
`,
			message: `unknown field "value"`,
		},
		{
			name: "invalid type",
			yaml: `
name: test
steps:
  - id: build
    run: echo ok
    outputs:
      - name: image_tag
        type: yaml
`,
			message: `type must be "string" or "json"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := spec.LoadYAML(
				context.Background(),
				[]byte(tc.yaml),
				spec.WithoutEval(),
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.message)
		})
	}
}

// Mechanisms that publish from captured output declare their names so strict
// step-output references can be validated without running the workflow.
func TestStepCapturedOutputsDeclaration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		yaml     string
		expected []ir.StepOutputDeclaration
	}{
		{
			name: "output schema",
			yaml: `
steps:
  - id: build
    run: echo ok
    output_schema:
      type: object
      properties:
        image: {type: string}
        meta: {type: object}
`,
			expected: []ir.StepOutputDeclaration{
				{Name: "image", Type: ir.StepDeclaredOutputTypeString, Source: ir.StepDeclaredOutputSourceCapture},
				{Name: "meta", Type: ir.StepDeclaredOutputTypeJSON, Source: ir.StepDeclaredOutputSourceCapture},
			},
		},
		{
			name: "object form output",
			yaml: `
steps:
  - id: build
    run: echo ok
    output:
      image:
        from: stdout
        decode: json
        select: .image
`,
			expected: []ir.StepOutputDeclaration{
				{Name: "image", Source: ir.StepDeclaredOutputSourceCapture},
			},
		},
		{
			name: "stdout outputs fields",
			yaml: `
steps:
  - id: build
    run: echo ok
    stdout:
      outputs:
        fields:
          image: {decode: json, select: .image}
`,
			expected: []ir.StepOutputDeclaration{
				{Name: "image", Source: ir.StepDeclaredOutputSourceCapture},
			},
		},
		{
			name: "outputs write values",
			yaml: `
steps:
  - id: build
    action: outputs.write
    with:
      values:
        image: v1
`,
			expected: []ir.StepOutputDeclaration{
				{Name: "image", Source: ir.StepDeclaredOutputSourceCapture},
			},
		},
		{
			name: "custom action output schema",
			yaml: `
actions:
  build.image:
    input_schema: {type: object, additionalProperties: false, properties: {}}
    output_schema:
      type: object
      properties:
        image: {type: string}
    template:
      run: echo ok
steps:
  - id: build
    action: build.image
    with: {}
`,
			expected: []ir.StepOutputDeclaration{
				{Name: "image", Type: ir.StepDeclaredOutputTypeString, Source: ir.StepDeclaredOutputSourceCapture},
			},
		},
		{
			name: "authored declaration wins over derived name",
			yaml: `
steps:
  - id: build
    run: echo ok
    outputs:
      - name: image
    output_schema:
      type: object
      properties:
        image: {type: string}
`,
			expected: []ir.StepOutputDeclaration{
				{Name: "image", Type: ir.StepDeclaredOutputTypeString},
			},
		},
		{
			name: "schema without inline properties declares nothing",
			yaml: `
steps:
  - id: build
    run: echo ok
    output_schema: {}
`,
			expected: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dag, err := spec.LoadYAML(context.Background(), []byte("name: test\n"+tc.yaml), spec.WithoutEval())
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			require.Equal(t, tc.expected, dag.Steps[0].Outputs)
		})
	}
}

// A step whose published names only a run reveals declares none, so nothing
// claims a contract the build cannot check.
func TestStepCapturedOutputsStayUndeclaredWhenDynamic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "stdout outputs decode whole object",
			yaml: `
steps:
  - id: build
    run: echo ok
    stdout:
      outputs:
        decode: json
`,
		},
		{
			name: "output schema without inline properties",
			yaml: `
steps:
  - id: build
    run: echo ok
    output_schema:
      $ref: '#/$defs/result'
      $defs:
        result:
          type: object
          properties:
            image: {type: string}
`,
		},
		{
			name: "unconstrained output schema",
			yaml: `
steps:
  - id: build
    run: echo ok
    output_schema: {}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dag, err := spec.LoadYAML(context.Background(), []byte("name: test\n"+tc.yaml), spec.WithoutEval())
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			require.Nil(t, dag.Steps[0].Outputs)
		})
	}
}
