package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue1661_CommandLikeString_WithSingleQuoteAfterVar(t *testing.T) {
	t.Parallel()

	scope := NewEnvScope(nil, false).WithEntry("MY_VALUE", "hello", EnvSourceDAGEnv)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "BracedVar",
			input: `nu -c "print $'got: ${MY_VALUE}'"`,
			want:  `nu -c "print $'got: hello'"`,
		},
		{
			name:  "SimpleVar",
			input: `nu -c "print $'got: $MY_VALUE'"`,
			want:  `nu -c "print $'got: hello'"`,
		},
		{
			name:  "MultipleVars",
			input: `nu -c "print $'bucket: ${BUCKET_PREFIX}${PROJECT_BUCKET}'"`,
			want:  `nu -c "print $'bucket: gs://my-bucket'"`,
		},
		{
			name:  "MissingVarPreserved",
			input: `nu -c "print $'got: ${MISSING}'"`,
			want:  `nu -c "print $'got: ${MISSING}'"`,
		},
	}

scope = scope.
		WithEntry("BUCKET_PREFIX", "gs://", EnvSourceDAGEnv).
		WithEntry("PROJECT_BUCKET", "my-bucket", EnvSourceDAGEnv)
	ctx := WithEnvScope(context.Background(), scope)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := String(ctx, tt.input, WithoutExpandEnv(), WithoutDollarEscape())
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
