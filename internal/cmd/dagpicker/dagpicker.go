package dagpicker

import (
	"context"
	"fmt"
	"io"
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
	Path string
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

// customDelegate implements list.ItemDelegate with custom rendering
type customDelegate struct{}

func (d customDelegate) Height() int                             { return 2 }
func (d customDelegate) Spacing() int                            { return 0 }
func (d customDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d customDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(DAGItem)
	if !ok {
		return
	}

	// Styles for selected/unselected items
	var nameStyle, descStyle lipgloss.Style
	prefix := "  "
	
	if index == m.Index() {
		nameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			MarginLeft(4)
		prefix = "> "
	} else {
		nameStyle = lipgloss.NewStyle()
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(4)
	}

	// Format the name line
	_, _ = fmt.Fprintf(w, "%s%s", prefix, nameStyle.Render(item.Name))
	
	// Add tags if present
	if len(item.Tags) > 0 {
		tagStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)
		_, _ = fmt.Fprintf(w, " %s", tagStyle.Render("["+strings.Join(item.Tags, ", ")+"]"))
	}
	_, _ = fmt.Fprintln(w)

	// Show description if available
	if item.Desc != "" {
		_, _ = fmt.Fprintf(w, "%s\n", descStyle.Render(item.Desc))
	} else {
		_, _ = fmt.Fprintln(w)
	}
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
			Path: "", // We don't need to show the path
			Desc: dag.Description,
			Tags: dag.Tags,
		})
	}

	// Create list with default delegate first to test
	l := list.New(items, list.NewDefaultDelegate(), 0, 0) // Let it auto-size
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
func PromptForParams(dag *digraph.DAG) ([]string, error) {
	if dag.DefaultParams == "" && len(dag.Params) == 0 {
		return nil, nil
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

	fmt.Print("\nEnter parameters (format: KEY=VALUE, press Enter to use defaults): ")

	var input string
	_, _ = fmt.Scanln(&input)

	if input == "" {
		return nil, nil
	}

	// Parse the input into key=value pairs
	params := strings.Fields(input)
	return params, nil
}
