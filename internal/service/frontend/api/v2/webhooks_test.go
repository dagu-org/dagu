package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractWebhookToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer my-secret-token",
			want:       "my-secret-token",
		},
		{
			name:       "empty header",
			authHeader: "",
			want:       "",
		},
		{
			name:       "no bearer prefix",
			authHeader: "my-secret-token",
			want:       "",
		},
		{
			name:       "wrong prefix",
			authHeader: "Basic my-secret-token",
			want:       "",
		},
		{
			name:       "bearer with extra spaces",
			authHeader: "Bearer  token-with-space",
			want:       " token-with-space",
		},
		{
			name:       "lowercase bearer",
			authHeader: "bearer my-token",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWebhookToken(tt.authHeader)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateWebhookToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provided string
		expected string
		want     bool
	}{
		{
			name:     "matching tokens",
			provided: "my-secret-token",
			expected: "my-secret-token",
			want:     true,
		},
		{
			name:     "non-matching tokens",
			provided: "wrong-token",
			expected: "my-secret-token",
			want:     false,
		},
		{
			name:     "empty provided",
			provided: "",
			expected: "my-secret-token",
			want:     false,
		},
		{
			name:     "empty expected",
			provided: "my-token",
			expected: "",
			want:     false,
		},
		{
			name:     "both empty",
			provided: "",
			expected: "",
			want:     false,
		},
		{
			name:     "tokens with special characters",
			provided: "token-with-$pecial-ch@rs!",
			expected: "token-with-$pecial-ch@rs!",
			want:     true,
		},
		{
			name:     "very long matching tokens",
			provided: "a-very-long-token-that-should-still-match-correctly-even-with-many-characters",
			expected: "a-very-long-token-that-should-still-match-correctly-even-with-many-characters",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateWebhookToken(tt.provided, tt.expected)
			require.Equal(t, tt.want, got)
		})
	}
}
