package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
)

func TestContains(t *testing.T) {
	f := &ContainTags{
		Tags: []string{"a", "b"},
	}
	cfg := &config.Config{
		Tags: []string{"a", "b", "c"},
	}
	require.True(t, f.Matches(cfg))
}
