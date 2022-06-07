package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestView(t *testing.T) {
	v := &View{
		Name:        "test",
		ContainTags: []string{"a", "b"},
	}
	js, err := v.ToJson()
	require.NoError(t, err)

	v2, err := ViewFromJson(js)
	require.NoError(t, err)
	require.Equal(t, v, v2)
}
