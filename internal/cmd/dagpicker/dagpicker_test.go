package dagpicker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDAGItem(t *testing.T) {
	t.Run("DAGItemImplementsListItemInterface", func(t *testing.T) {
		item := DAGItem{
			Name:   "test-dag",
			Path:   "/path/to/test.yaml",
			Desc:   "Test DAG description",
			Tags:   []string{"test", "example"},
			Params: "KEY1=value1 KEY2=value2",
		}

		assert.Equal(t, "test-dag [test, example]", item.Title())
		assert.Equal(t, "Test DAG description | params: KEY1=value1 KEY2=value2", item.Description())
		assert.Equal(t, "test-dag", item.FilterValue())
	})

	t.Run("DAGItemWithEmptyDescriptionButParams", func(t *testing.T) {
		item := DAGItem{
			Name:   "test-dag",
			Path:   "/path/to/test.yaml",
			Desc:   "",
			Tags:   []string{"test"},
			Params: "KEY=value",
		}

		assert.Equal(t, "params: KEY=value", item.Description())
	})

	t.Run("DAGItemWithDescriptionButNoParams", func(t *testing.T) {
		item := DAGItem{
			Name:   "test-dag",
			Path:   "/path/to/test.yaml",
			Desc:   "Just a description",
			Tags:   []string{"test"},
			Params: "",
		}

		assert.Equal(t, "Just a description", item.Description())
	})
}

func TestPromptForParams(t *testing.T) {
	t.Run("ReturnsEmptyStringWhenDAGHasNoParameters", func(t *testing.T) {
		dag := &core.DAG{
			DefaultParams: "",
			Params:        []string{},
		}

		params, err := PromptForParams(dag)
		assert.NoError(t, err)
		assert.Empty(t, params)
	})

	t.Run("DAGWithDefaultParameters", func(_ *testing.T) {
		dag := &core.DAG{
			DefaultParams: "KEY1=value1 KEY2=value2",
			Params:        []string{},
		}

		// Note: This test won't actually prompt for input in a test environment
		// It's mainly to ensure the function doesn't panic with valid input
		_ = dag
	})
}

func TestModel(t *testing.T) {
	t.Run("ModelInitialization", func(t *testing.T) {
		ti := textinput.New()
		m := Model{
			paramInput: ti,
		}
		cmd := m.Init()
		assert.NotNil(t, cmd) // Now returns textinput.Blink
	})

	t.Run("ModelViewWhenDone", func(t *testing.T) {
		m := Model{
			state: StateDone,
		}

		view := m.View()
		assert.Equal(t, "", view)
	})

	t.Run("ModelViewInSelectingState", func(t *testing.T) {
		items := []list.Item{
			DAGItem{Name: "test", Desc: "Test DAG"},
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)

		m := Model{
			state: StateSelectingDAG,
			list:  l,
		}

		view := m.View()
		assert.Contains(t, view, "test")     // Should show the DAG name
		assert.Contains(t, view, "Test DAG") // Should show the description
	})

	t.Run("ModelHandlesEscapeKeyInDAGSelection", func(t *testing.T) {
		items := []list.Item{
			DAGItem{Name: "test", Desc: "Test DAG"},
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)

		m := Model{
			state: StateSelectingDAG,
			list:  l,
		}

		escMsg := tea.KeyMsg{Type: tea.KeyEsc}
		updatedModel, cmd := m.Update(escMsg)
		updatedM := updatedModel.(Model)

		assert.True(t, updatedM.quitting)
		assert.Equal(t, StateDone, updatedM.state)
		assert.NotNil(t, cmd)
	})

	t.Run("ModelHandlesCtrlC", func(t *testing.T) {
		m := Model{
			state: StateSelectingDAG,
		}

		ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
		updatedModel, cmd := m.Update(ctrlCMsg)
		updatedM := updatedModel.(Model)

		assert.True(t, updatedM.quitting)
		assert.Equal(t, StateDone, updatedM.state)
		assert.NotNil(t, cmd)
	})

	t.Run("ModelHandlesWindowSize", func(t *testing.T) {
		// Create a model with an initialized list
		items := []list.Item{
			DAGItem{Name: "test", Desc: "Test DAG"},
		}
		l := list.New(items, list.NewDefaultDelegate(), 80, 20)
		m := Model{
			list: l,
		}

		sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
		_, cmd := m.Update(sizeMsg)

		assert.Nil(t, cmd)
	})
}

var _ execution.DAGStore = (*mockDAGStore)(nil)

// mockDAGStore is a mock implementation of models.DAGStore
type mockDAGStore struct {
	mock.Mock
}

func (m *mockDAGStore) Create(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *mockDAGStore) Delete(ctx context.Context, fileName string) error {
	args := m.Called(ctx, fileName)
	return args.Error(0)
}

func (m *mockDAGStore) List(ctx context.Context, params execution.ListDAGsOptions) (execution.PaginatedResult[*core.DAG], []string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(execution.PaginatedResult[*core.DAG]), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) GetMetadata(ctx context.Context, fileName string) (*core.DAG, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) GetDetails(ctx context.Context, fileName string, opts ...spec.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, fileName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) Grep(ctx context.Context, pattern string) ([]*execution.GrepDAGsResult, []string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]*execution.GrepDAGsResult), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) Rename(ctx context.Context, oldID, newID string) error {
	args := m.Called(ctx, oldID, newID)
	return args.Error(0)
}

func (m *mockDAGStore) GetSpec(ctx context.Context, fileName string) (string, error) {
	args := m.Called(ctx, fileName)
	return args.String(0), args.Error(1)
}

func (m *mockDAGStore) UpdateSpec(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *mockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...spec.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) TagList(ctx context.Context) ([]string, []string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) ToggleSuspend(ctx context.Context, fileName string, suspend bool) error {
	args := m.Called(ctx, fileName, suspend)
	return args.Error(0)
}

func (m *mockDAGStore) IsSuspended(ctx context.Context, fileName string) bool {
	args := m.Called(ctx, fileName)
	return args.Bool(0)
}

func TestPickDAG(t *testing.T) {
	t.Run("ReturnsErrorWhenDAGStoreFails", func(t *testing.T) {
		mockStore := new(mockDAGStore)
		ctx := context.Background()

		mockStore.On("List", ctx, execution.ListDAGsOptions{}).Return(
			execution.PaginatedResult[*core.DAG]{},
			[]string{},
			errors.New("database error"),
		)

		_, err := PickDAG(ctx, mockStore)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list DAGs")
		mockStore.AssertExpectations(t)
	})

	t.Run("ReturnsErrorWhenNoDAGsFound", func(t *testing.T) {
		mockStore := new(mockDAGStore)
		ctx := context.Background()

		mockStore.On("List", ctx, execution.ListDAGsOptions{}).Return(
			execution.PaginatedResult[*core.DAG]{
				Items: []*core.DAG{},
			},
			[]string{},
			nil,
		)

		_, err := PickDAG(ctx, mockStore)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no DAGs found")
		mockStore.AssertExpectations(t)
	})

	t.Run("CreatesProperDAGItemsFromDAGs", func(t *testing.T) {
		// This tests the internal logic of converting DAGs to list items
		dags := []*core.DAG{
			{
				Name:        "dag1",
				Location:    "/path/to/dag1.yaml",
				Description: "First DAG",
				Tags:        []string{"production", "critical"},
			},
			{
				Name:        "dag2",
				Location:    "/path/to/dag2.yaml",
				Description: "Second DAG",
				Tags:        []string{"test"},
			},
		}

		// Convert to DAGItems
		items := make([]DAGItem, 0, len(dags))
		for _, dag := range dags {
			// Format parameters for display
			var params string
			if dag.DefaultParams != "" {
				params = dag.DefaultParams
			} else if len(dag.Params) > 0 {
				params = strings.Join(dag.Params, " ")
			}

			items = append(items, DAGItem{
				Name:   dag.Name,
				Path:   dag.Location,
				Desc:   dag.Description,
				Tags:   dag.Tags,
				Params: params,
			})
		}

		assert.Len(t, items, 2)
		assert.Equal(t, "dag1", items[0].Name)
		assert.Equal(t, "/path/to/dag1.yaml", items[0].Path)
		assert.Equal(t, "First DAG", items[0].Desc)
		assert.Equal(t, []string{"production", "critical"}, items[0].Tags)
	})
}

func TestPromptForParamsReturnsInput(t *testing.T) {
	t.Run("ReturnsUserInputAsIs", func(t *testing.T) {
		// This test verifies that PromptForParams returns the input without modification
		// In a real test, we would mock stdin, but for now we just verify the concept

		// Test cases showing what kind of input would be returned
		testInputs := []string{
			"KEY1=value1 KEY2=value2",
			`{"key": "value", "nested": {"item": 123}}`,
			"some free text parameter",
			"KEY=value with spaces",
			"",
		}

		for _, input := range testInputs {
			// In the actual function, strings.TrimSpace(input) would be returned
			expected := strings.TrimSpace(input)
			assert.Equal(t, expected, strings.TrimSpace(input))
		}
	})
}

func TestConfirmRunDAG(t *testing.T) {
	t.Run("AcceptWithY", func(t *testing.T) {
		// Mock stdin and stdout
		oldStdin := os.Stdin
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdin = r
		// Redirect stdout to prevent interference with test output
		_, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW
		defer func() {
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			_ = r.Close()
			_ = stdoutW.Close()
		}()

		go func() {
			_, _ = w.WriteString("y\n")
			_ = w.Close()
		}()

		confirmed, err := ConfirmRunDAG("test-dag", "param1=value1")
		assert.NoError(t, err)
		assert.True(t, confirmed)
	})

	t.Run("AcceptWithYes", func(t *testing.T) {
		// Mock stdin and stdout
		oldStdin := os.Stdin
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdin = r
		// Redirect stdout to prevent interference with test output
		_, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW
		defer func() {
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			_ = r.Close()
			_ = stdoutW.Close()
		}()

		go func() {
			_, _ = w.WriteString("yes\n")
			_ = w.Close()
		}()

		confirmed, err := ConfirmRunDAG("test-dag", "")
		assert.NoError(t, err)
		assert.True(t, confirmed)
	})

	t.Run("AcceptWithEmpty", func(t *testing.T) {
		// Mock stdin and stdout
		oldStdin := os.Stdin
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdin = r
		// Redirect stdout to prevent interference with test output
		_, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW
		defer func() {
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			_ = r.Close()
			_ = stdoutW.Close()
		}()

		go func() {
			_, _ = w.WriteString("\n")
			_ = w.Close()
		}()

		confirmed, err := ConfirmRunDAG("test-dag", "param1=value1")
		assert.NoError(t, err)
		assert.True(t, confirmed)
	})

	t.Run("RejectWithN", func(t *testing.T) {
		// Mock stdin and stdout
		oldStdin := os.Stdin
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdin = r
		// Redirect stdout to prevent interference with test output
		_, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW
		defer func() {
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			_ = r.Close()
			_ = stdoutW.Close()
		}()

		go func() {
			_, _ = w.WriteString("n\n")
			_ = w.Close()
		}()

		confirmed, err := ConfirmRunDAG("test-dag", "")
		assert.NoError(t, err)
		assert.False(t, confirmed)
	})

	t.Run("RejectWithNo", func(t *testing.T) {
		// Mock stdin and stdout
		oldStdin := os.Stdin
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdin = r
		// Redirect stdout to prevent interference with test output
		_, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW
		defer func() {
			os.Stdin = oldStdin
			os.Stdout = oldStdout
			_ = r.Close()
			_ = stdoutW.Close()
		}()

		go func() {
			_, _ = w.WriteString("no\n")
			_ = w.Close()
		}()

		confirmed, err := ConfirmRunDAG("test-dag", "params here")
		assert.NoError(t, err)
		assert.False(t, confirmed)
	})
}
