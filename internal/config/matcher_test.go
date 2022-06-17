package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContains(t *testing.T) {
	f := &ContainTagsMatcher{
		Tags: []string{"a", "b"},
	}
	cfg := &Config{
		Tags: []string{"a", "b", "c"},
	}
	require.True(t, f.Matches(cfg))
}
