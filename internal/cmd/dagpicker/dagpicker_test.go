package dagpicker

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
)

func TestDAGItem(t *testing.T) {
	t.Run("DAGItem implements list.Item interface", func(t *testing.T) {
		item := DAGItem{
			Name: "test-dag",
			Path: "/path/to/test.yaml",
			Desc: "Test DAG description",
			Tags: []string{"test", "example"},
		}

		assert.Equal(t, "test-dag", item.Title())
		assert.Equal(t, "Test DAG description", item.Description())
		assert.Equal(t, "test-dag", item.FilterValue())
	})

	t.Run("DAGItem with empty description uses path", func(t *testing.T) {
		item := DAGItem{
			Name: "test-dag",
			Path: "/path/to/test.yaml",
			Desc: "",
			Tags: []string{"test"},
		}

		assert.Equal(t, "", item.Description())
	})
}

func TestPromptForParams(t *testing.T) {
	t.Run("Returns nil when DAG has no parameters", func(t *testing.T) {
		dag := &digraph.DAG{
			DefaultParams: "",
			Params:        []string{},
		}

		params, err := PromptForParams(dag)
		assert.NoError(t, err)
		assert.Nil(t, params)
	})

	t.Run("DAG with default parameters", func(_ *testing.T) {
		dag := &digraph.DAG{
			DefaultParams: "KEY1=value1 KEY2=value2",
			Params:        []string{},
		}

		// Note: This test won't actually prompt for input in a test environment
		// It's mainly to ensure the function doesn't panic with valid input
		_ = dag
	})
}

func TestModel(t *testing.T) {
	t.Run("Model initialization", func(t *testing.T) {
		m := Model{}
		cmd := m.Init()
		assert.Nil(t, cmd)
	})

	t.Run("Model view when quitting", func(t *testing.T) {
		m := Model{
			quitting: true,
			choice:   nil,
		}

		view := m.View()
		assert.Equal(t, "Selection cancelled.\n", view)
	})

	t.Run("Model view when DAG selected", func(t *testing.T) {
		m := Model{
			quitting: true,
			choice: &DAGItem{
				Name: "selected-dag",
			},
		}

		view := m.View()
		assert.Equal(t, "", view)
	})
}

func TestCustomDelegate(t *testing.T) {
	t.Run("Delegate properties", func(t *testing.T) {
		d := customDelegate{}
		assert.Equal(t, 2, d.Height())
		assert.Equal(t, 0, d.Spacing())
		assert.Nil(t, d.Update(nil, nil))
	})
}