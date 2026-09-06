// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec069_secrets_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// The env provider resolves from the real OS environment. The tree preview
// is read back from the on-disk step log file, so the resolved value being
// masked there proves the masking writer redacts the file itself, not just
// the terminal display.
func TestEnvProviderMasksValue(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv([]string{"SOURCE_SECRET_VALUE=supersecret123"}, "start", "env_success.yaml")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), "*******")
	require.NotContains(t, result.Stdout(), "supersecret123")
}

func TestEnvProviderMissingVariable(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "env_missing.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains(`environment variable "DAGU_CONFORMANCE_UNSET_SECRET_VAR" is not set`)
}

// The file provider resolves relative paths against the DAG's own directory.
func TestFileProviderResolvesAndMasksValue(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "file_success.yaml")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), "*******")
	require.NotContains(t, result.Stdout(), "filesecretvalue123")
}

func TestFileProviderMissingFile(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "file_missing.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("secret file not found")
}

func TestUnknownProvider(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "unknown_provider.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("unknown secret provider: nope")
}

// Build-time validation rejects malformed secret declarations before any
// provider is contacted.
func TestValidateRejectsMalformedSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		errText string
	}{
		{"validate_missing_name.yaml", "'name' field is required"},
		{"validate_invalid_name.yaml", "must be a valid environment variable name"},
		{"validate_dagu_prefix.yaml", "must not start with DAGU_"},
		{"validate_reserved_name.yaml", "collides with Dagu-managed runtime environment variable"},
		{"validate_duplicate_name.yaml", `duplicate secret name "MY_SECRET"`},
		{"validate_ref_and_provider.yaml", "exactly one of 'ref' or 'provider' plus 'key' is required"},
		{"validate_neither.yaml", "exactly one of 'ref' or 'provider' plus 'key' is required"},
		{"validate_options_with_ref.yaml", "'options' cannot be used with registry ref"},
		{"validate_bad_ref_pattern.yaml", "registry ref must be a slash-separated lowercase slug path"},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("validate", tc.fixture)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.errText)
		})
	}
}

// Providers whose parsing fails before any network client is created report
// a provider-specific diagnostic. No provider here ever dials out.
func TestProviderSpecificValidationFailsWithoutNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		errText string
	}{
		{"gcp_missing_project.yaml", `project ID is required for GCP Secret Manager secret "bare-id-no-options"`},
		{"azure_bad_option.yaml", `unsupported option "bogus"`},
		{"alibaba_bad_name.yaml", "secret name for Alibaba Cloud KMS contains unsupported characters"},
		{"kubernetes_bad_key.yaml", "secret name and data key are required"},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("start", tc.fixture)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.errText)
		})
	}
}

// The AWS Secrets Manager provider uses the standard AWS SDK, which honors
// AWS_ENDPOINT_URL. A local mock server proves the real client, request
// signing, and JSON 1.1 response parsing all work end-to-end.
func TestAWSProviderLiveResolve(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "secretsmanager.GetSecretValue", r.Header.Get("X-Amz-Target"))
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_, _ = w.Write([]byte(`{"Name":"mysecret","SecretString":"awssecretvalue456"}`))
	}))
	t.Cleanup(server.Close)

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(awsMockEnv(server.URL), "start", "aws.yaml")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), "*******")
	require.NotContains(t, result.Stdout(), "awssecretvalue456")
}

func TestAWSProviderNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.Header().Set("X-Amzn-Errortype", "ResourceNotFoundException")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Message":"Secrets Manager can't find the specified secret."}`))
	}))
	t.Cleanup(server.Close)

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(awsMockEnv(server.URL), "start", "aws.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains(`AWS Secrets Manager secret "mysecret" was not found`)
}

func awsMockEnv(endpoint string) []string {
	return []string{
		"AWS_ENDPOINT_URL=" + endpoint,
		"AWS_ACCESS_KEY_ID=conformance-test",
		"AWS_SECRET_ACCESS_KEY=conformance-test",
		"AWS_REGION=us-east-1",
	}
}
