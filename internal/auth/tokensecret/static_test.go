package tokensecret_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/auth/tokensecret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticProvider(t *testing.T) {
	t.Run("valid secret", func(t *testing.T) {
		p, err := tokensecret.NewStatic("my-secret")
		require.NoError(t, err)

		ts, err := p.Resolve(context.Background())
		require.NoError(t, err)
		assert.True(t, ts.IsValid())
		assert.Equal(t, []byte("my-secret"), ts.SigningKey())
	})

	t.Run("empty secret", func(t *testing.T) {
		_, err := tokensecret.NewStatic("")
		assert.ErrorIs(t, err, auth.ErrInvalidTokenSecret)
	})
}
