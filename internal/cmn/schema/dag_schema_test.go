// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package schema

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDAGSchemaParams(t *testing.T) {
	t.Parallel()

	resolved := mustResolveDAGSchema(t)

	tests := []struct {
		name    string
		spec    string
		wantErr string
	}{
		{
			name: "StringPositionalParams",
			spec: `
params: first second
steps:
  - command: echo "$1 $2"
`,
		},
		{
			name: "LegacyNamedList",
			spec: `
params:
  - ENVIRONMENT: prod
  - COUNT: 3
steps:
  - command: echo "${ENVIRONMENT} ${COUNT}"
`,
		},
		{
			name: "InlineRichParams",
			spec: `
params:
  - name: region
    type: string
    default: us-east-1
    enum: [us-east-1, us-west-2]
    description: Deployment region
  - name: count
    type: integer
    default: 3
    minimum: 1
    maximum: 10
  - name: debug
    type: boolean
    default: false
steps:
  - command: echo "${region} ${count} ${debug}"
`,
		},
		{
			name: "MixedLegacyAndInline",
			spec: `
params:
  - name: environment
    type: string
    default: staging
    enum: [dev, staging, prod]
  - TAG: latest
steps:
  - command: echo "${environment} ${TAG}"
`,
		},
		{
			name: "ExternalSchemaMode",
			spec: `
params:
  schema: ./params.schema.json
  values:
    batch_size: 25
    environment: staging
steps:
  - command: echo done
`,
		},
		{
			name: "RejectCamelCaseInlineField",
			spec: `
params:
  - name: project_name
    type: string
    minLength: 3
steps:
  - command: echo "${project_name}"
`,
			wantErr: "params",
		},
		{
			name: "RejectLegacyNestedMapInlineEntry",
			spec: `
params:
  - project_name:
      type: string
      default: demo
steps:
  - command: echo hi
`,
			wantErr: "params",
		},
		{
			name: "RejectNameOnlyRichEntry",
			spec: `
params:
  - name: foo
steps:
  - command: echo "${foo}"
`,
			wantErr: "params",
		},
		{
			name: "LegacyMapAllowsSchemaKey",
			spec: `
params:
  schema: prod
  region: us
steps:
  - command: echo "${schema} ${region}"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc := mustParseYAMLDocument(t, tt.spec)
			err := resolved.Validate(doc)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDAGSchemaRootRetryPolicy(t *testing.T) {
	t.Parallel()

	resolved := mustResolveDAGSchema(t)

	tests := []struct {
		name    string
		spec    string
		wantErr string
	}{
		{
			name: "NumericValues",
			spec: `
name: retryable-dag
retry_policy:
  limit: 3
  interval_sec: 10
  backoff: 2.0
  max_interval_sec: 60
steps:
  - command: echo hi
`,
		},
		{
			name: "NumericStringsAndBooleanBackoff",
			spec: `
name: retryable-dag
retry_policy:
  limit: "03"
  interval_sec: "10"
  backoff: false
  max_interval_sec: "60"
steps:
  - command: echo hi
`,
		},
		{
			name: "RejectsMissingLimit",
			spec: `
name: retryable-dag
retry_policy:
  interval_sec: 10
steps:
  - command: echo hi
`,
			wantErr: "retry_policy",
		},
		{
			name: "RejectsNonNumericStringLimit",
			spec: `
name: retryable-dag
retry_policy:
  limit: three
  interval_sec: 10
steps:
  - command: echo hi
`,
			wantErr: "retry_policy",
		},
		{
			name: "RejectsNonNumericStringInterval",
			spec: `
name: retryable-dag
retry_policy:
  limit: 3
  interval_sec: later
steps:
  - command: echo hi
`,
			wantErr: "retry_policy",
		},
		{
			name: "RejectsBackoffOnePointZero",
			spec: `
name: retryable-dag
retry_policy:
  limit: 3
  interval_sec: 10
  backoff: 1.0
steps:
  - command: echo hi
`,
			wantErr: "retry_policy",
		},
		{
			name: "RejectsUnknownRetryField",
			spec: `
name: retryable-dag
retry_policy:
  limit: 3
  unknown_retry_field: 10
steps:
  - command: echo hi
`,
			wantErr: "retry_policy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc := mustParseYAMLDocument(t, tt.spec)
			err := resolved.Validate(doc)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDAGSchemaRootRetryPolicyRejectsExitCode(t *testing.T) {
	t.Parallel()

	resolved := mustResolveDAGSchema(t)
	doc := mustParseYAMLDocument(t, `
name: retryable-dag
retry_policy:
  limit: 3
  interval_sec: 10
  exit_code: [1]
steps:
  - command: echo hi
`)

	err := resolved.Validate(doc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "retry_policy")
}

func TestDAGSchemaStepRetryPolicyRejectsUnknownField(t *testing.T) {
	t.Parallel()

	resolved := mustResolveDAGSchema(t)
	doc := mustParseYAMLDocument(t, `
steps:
  - command: echo hi
    retry_policy:
      limit: 1
      interval_sec: 5
      unknown_retry_field: 2
`)

	err := resolved.Validate(doc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "steps")
}

func mustResolveDAGSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()

	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(DAGSchemaJSON, &schema))

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{})
	require.NoError(t, err)
	return resolved
}

func mustParseYAMLDocument(t *testing.T, spec string) map[string]any {
	t.Helper()

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	return doc
}
