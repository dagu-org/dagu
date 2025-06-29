package dagpicker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

// DAGItem represents a DAG in the list
type DAGItem struct {
	Name string
	Path string // Path is stored but not displayed
	Desc string
	Tags []string
}

func (i DAGItem) Title() string       { return i.Name }
func (i DAGItem) Description() string { return i.Desc }
func (i DAGItem) FilterValue() string { return i.Name }

// Model represents the state of the DAG picker
type Model struct {
	list     list.Model
	choice   *DAGItem
	quitting bool
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(DAGItem); ok {
				m.choice = &item
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the UI
func (m Model) View() string {
	if m.quitting {
		if m.choice != nil {
			return ""
		}
		return "Selection cancelled.\n"
	}
	return docStyle.Render(m.list.View())
}

// PickDAG shows an interactive DAG picker and returns the selected DAG path
func PickDAG(ctx context.Context, dagStore models.DAGStore) (string, error) {
	// Get list of DAGs
	result, errs, err := dagStore.List(ctx, models.ListDAGsOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list DAGs: %w", err)
	}

	if len(errs) > 0 {
		// Log errors but continue
		for _, e := range errs {
			fmt.Printf("Warning: %s\n", e)
		}
	}

	if len(result.Items) == 0 {
		return "", fmt.Errorf("no DAGs found in the configured directory")
	}

	// Convert DAGs to list items
	items := make([]list.Item, 0, len(result.Items))
	for _, dag := range result.Items {
		items = append(items, DAGItem{
			Name: dag.Name,
			Path: dag.Location,
			Desc: dag.Description,
			Tags: dag.Tags,
		})
	}

	// Create list with custom delegate for better rendering
	l := list.New(items, list.NewDefaultDelegate(), 80, 20) // Default size for reasonable display
	l.Title = "Select a DAG to run"
	l.SetShowStatusBar(true)
	l.SetStatusBarItemName("DAG", "DAGs")
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.SetShowTitle(true)
	l.DisableQuitKeybindings()

	// Style the title
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#6B46C1")).
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	m := Model{
		list: l,
	}

	// Run the picker
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run DAG picker: %w", err)
	}

	// Get the selection
	finalM, ok := finalModel.(Model)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}

	if finalM.choice == nil {
		return "", fmt.Errorf("no DAG selected")
	}

	// Return the DAG name (which will be resolved by the loader)
	return finalM.choice.Name, nil
}

// PromptForParams prompts the user to enter parameters for a DAG
func PromptForParams(dag *digraph.DAG) (string, error) {
	if dag.DefaultParams == "" && len(dag.Params) == 0 {
		return "", nil
	}

	fmt.Println("\nThis DAG accepts the following parameters:")

	// Display default parameters if available
	if dag.DefaultParams != "" {
		fmt.Printf("  Default: %s\n", dag.DefaultParams)
	}

	// Display current parameters if available
	if len(dag.Params) > 0 {
		fmt.Printf("  Current: %s\n", strings.Join(dag.Params, " "))
	}

	fmt.Print("\nEnter parameters (press Enter to use defaults): ")

	// Read full line of input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	// Trim whitespace and return
	return strings.TrimSpace(input), nil
}
