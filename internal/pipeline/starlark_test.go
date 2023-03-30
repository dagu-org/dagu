package pipeline

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	"testing"
)

func TestNewPipeline(t *testing.T) {
	g, err := NewPipeline("data/home.star")
	require.NoError(t, err)
	fmt.Println(g)
}

func TestEdgesToDependencyMap(t *testing.T) {
	edges := starlark.NewList([]starlark.Value{
		starlark.Tuple{
			starlark.String("a"),
			starlark.String("b"),
		},
		starlark.Tuple{
			starlark.String("b"),
			starlark.String("c"),
		},
		starlark.Tuple{
			starlark.String("a"),
			starlark.String("c"),
		},
	})
	dependencyMap := EdgesToDependencyMap(edges)
	expected := map[string][]string{
		"b": {"a"},
		"c": {"b", "a"},
	}
	require.Equal(t, expected, dependencyMap)
}
