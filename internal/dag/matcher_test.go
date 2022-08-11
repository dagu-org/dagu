package dag

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContains(t *testing.T) {
	f := &ContainTagsMatcher{
		Tags: []string{"a", "b"},
	}
	cfg := &DAG{
		Tags: []string{"a", "b", "c"},
	}
	require.True(t, f.Matches(cfg))
}
