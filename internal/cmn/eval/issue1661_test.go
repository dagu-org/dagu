package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssue1661_CommandLikeString_WithSingleQuoteAfterVar(t *testing.T) {
	t.Parallel()

	scope := NewEnvScope(nil, false).WithEntry("MY_VALUE", "hello", EnvSourceDAGEnv)
	ctx := WithEnvScope(context.Background(), scope)

	input := `nu -c "print $'got: ${MY_VALUE}'"`
	got, err := String(ctx, input, WithoutExpandEnv(), WithoutDollarEscape())

	require.NoError(t, err)
	require.Equal(t, `nu -c "print $'got: hello'"`, got)
}
