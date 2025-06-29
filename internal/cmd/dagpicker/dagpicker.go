package dagpicker

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	Name   string
	Path   string // Path is stored but not displayed
	Desc   string
	Tags   []string
	Params string // Parameters that the DAG accepts
}

func (i DAGItem) Title() string { 
	title := i.Name
	if len(i.Tags) > 0 {
		title += " [" + strings.Join(i.Tags, ", ") + "]"
	}
	return title
}
func (i DAGItem) Description() string {
	desc := i.Desc
	if i.Params != "" {
		if desc != "" {
			desc += " • "
		}
		desc += "params: " + i.Params
	}
	return desc
}
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

func (d customDelegate) Height() int                             { return 3 } // Increased height for params
func (d customDelegate) Spacing() int                            { return 0 }
func (d customDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d customDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(DAGItem)
	if !ok {
		return
	}

	// Buffer output to ensure atomic writes
	var buf strings.Builder

	// Styles for selected/unselected items
	var nameStyle, descStyle, paramStyle, tagStyle lipgloss.Style
	prefix := "  "

	if index == m.Index() {
		nameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			MarginLeft(4)
		paramStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			MarginLeft(4).
			Italic(true)
		tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)
		prefix = "> "
	} else {
		nameStyle = lipgloss.NewStyle()
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(4)
		paramStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			MarginLeft(4).
			Italic(true)
		tagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
	}

	// Format the name line
	buf.WriteString(prefix)
	buf.WriteString(nameStyle.Render(item.Name))

	// Add tags if present
	if len(item.Tags) > 0 {
		buf.WriteString(" ")
		buf.WriteString(tagStyle.Render("[" + strings.Join(item.Tags, ", ") + "]"))
	}
	buf.WriteString("\n")

	// Show description if available
	if item.Desc != "" {
		buf.WriteString(descStyle.Render(item.Desc))
		buf.WriteString("\n")
	}

	// Show parameters if available
	if item.Params != "" {
		buf.WriteString(paramStyle.Render("params: " + item.Params))
		buf.WriteString("\n")
	}

	// Write buffered output
	_, _ = w.Write([]byte(buf.String()))
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

	// Create list with custom delegate for better rendering
	delegate := customDelegate{}
	l := list.New(items, delegate, 80, 20) // Default size for reasonable display
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
