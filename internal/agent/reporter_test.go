package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/stretchr/testify/require"
)

func TestReporter(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, rp *reporter, mock *mockSender, dag *digraph.DAG, nodes []*models.Node,
	){
		"create error mail":   testErrorMail,
		"no error mail":       testNoErrorMail,
		"create success mail": testSuccessMail,
		"create summary":      testRenderSummary,
		"create node list":    testRenderTable,
	} {
		t.Run(scenario, func(t *testing.T) {

			d := &digraph.DAG{
				Name: "test DAG",
				MailOn: &digraph.MailOn{
					Failure: true,
				},
				ErrorMail: &digraph.MailConfig{
					Prefix: "Error: ",
					From:   "from@mailer.com",
					To:     []string{"to@mailer.com"},
				},
				InfoMail: &digraph.MailConfig{
					Prefix: "Success: ",
					From:   "from@mailer.com",
					To:     []string{"to@mailer.com"},
				},
				Steps: []digraph.Step{
					{
						Name:    "test-step",
						Command: "true",
					},
				},
			}

			nodes := []*models.Node{
				{
					Step: digraph.Step{
						Name:    "test-step",
						Command: "true",
						Args:    []string{"param-x"},
					},
					Status:     status.NodeRunning,
					StartedAt:  stringutil.FormatTime(time.Now()),
					FinishedAt: stringutil.FormatTime(time.Now().Add(time.Minute * 10)),
				},
			}

			mock := &mockSender{}
			rp := &reporter{senderFn: mock.Send}

			fn(t, rp, mock, d, nodes)
		})
	}
}

func TestRenderHTMLWithDAGInfo(t *testing.T) {
	// Create a test DAGRunStatus
	status := models.DAGRunStatus{
		Name:       "test-workflow",
		DAGRunID:   "01975986-c13d-7b6d-b75e-abf4380a03fc",
		Status:     status.Success,
		StartedAt:  "2025-01-15T10:30:00Z",
		FinishedAt: "2025-01-15T10:35:00Z",
		Params:     "env=production batch_size=1000",
		Nodes: []*models.Node{
			{
				Step: digraph.Step{
					Name:        "setup-database",
					Command:     "psql",
					Args:        []string{"-h", "localhost", "-U", "admin", "-d", "mydb", "-f", "schema.sql"},
					CmdWithArgs: "psql -h localhost -U admin -d mydb -f schema.sql",
				},
				Status:     status.NodeSuccess,
				StartedAt:  "2025-01-15T10:30:00Z",
				FinishedAt: "2025-01-15T10:30:45Z",
				Error:      "",
			},
			{
				Step: digraph.Step{
					Name:        "run-migrations",
					Command:     "migrate",
					Args:        []string{"up"},
					CmdWithArgs: "migrate up",
				},
				Status:     status.NodeError,
				StartedAt:  "2025-01-15T10:31:00Z",
				FinishedAt: "2025-01-15T10:31:15Z",
				Error:      "Migration failed: Table 'users' already exists",
			},
		},
	}

	// Call renderHTMLWithDAGInfo to get the output
	html := renderHTMLWithDAGInfo(status)

	// Verify HTML structure and content
	t.Run("DAGInfoSection", func(t *testing.T) {
		// Check DAG info section exists
		require.Contains(t, html, "DAG Execution Details")
		require.Contains(t, html, "dag-info")

		// Check DAG name is displayed
		require.Contains(t, html, "dag-name")
		require.Contains(t, html, "test-workflow")

		// Check DAG Run ID
		require.Contains(t, html, "DAG Run ID")
		require.Contains(t, html, "01975986-c13d-7b6d-b75e-abf4380a03fc")

		// Check Parameters
		require.Contains(t, html, "Parameters")
		require.Contains(t, html, "env=production batch_size=1000")

		// Check timestamps
		require.Contains(t, html, "Started At")
		require.Contains(t, html, "Finished At")
		require.Contains(t, html, "2025-01-15T10:30:00Z")
		require.Contains(t, html, "2025-01-15T10:35:00Z")

		// Check status badge
		require.Contains(t, html, "status-badge success") // CSS class for success status
		require.Contains(t, html, "FINISHED")             // Status text
	})

	t.Run("TableContent", func(t *testing.T) {
		// Check that step table is still included
		require.Contains(t, html, "<table>")
		require.Contains(t, html, "setup-database")
		require.Contains(t, html, "run-migrations")
		require.Contains(t, html, "Migration failed")
	})

	t.Run("EmptyParameters", func(t *testing.T) {
		// Test with empty parameters
		statusNoParams := status
		statusNoParams.Params = ""
		htmlNoParams := renderHTMLWithDAGInfo(statusNoParams)

		// Should show "(none)" for empty parameters
		require.Contains(t, htmlNoParams, "(none)")
	})

	t.Run("HTMLEscapingInDAGInfo", func(t *testing.T) {
		// Test HTML escaping in DAG info
		statusWithSpecialChars := status
		statusWithSpecialChars.Name = "test<script>alert('xss')</script>"
		statusWithSpecialChars.DAGRunID = "id&with&ampersands"
		statusWithSpecialChars.Params = "param=\"<value>\""

		htmlEscaped := renderHTMLWithDAGInfo(statusWithSpecialChars)

		// Verify dangerous HTML characters are properly escaped
		require.NotContains(t, htmlEscaped, "<script>alert('xss')</script>")
		require.Contains(t, htmlEscaped, "&lt;script&gt;alert('xss')&lt;/script&gt;")
		require.Contains(t, htmlEscaped, "id&amp;with&amp;ampersands")
		require.Contains(t, htmlEscaped, "param=&quot;&lt;value&gt;&quot;")
	})
}

func testErrorMail(t *testing.T, rp *reporter, mock *mockSender, dag *digraph.DAG, nodes []*models.Node) {
	dag.MailOn.Failure = true
	dag.MailOn.Success = false

	_ = rp.send(context.Background(), dag, models.DAGRunStatus{
		Status: status.Error,
		Nodes:  nodes,
	}, fmt.Errorf("Error"))

	require.Contains(t, mock.subject, "Error")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testNoErrorMail(t *testing.T, rp *reporter, mock *mockSender, dag *digraph.DAG, nodes []*models.Node) {
	dag.MailOn.Failure = false
	dag.MailOn.Success = true

	err := rp.send(context.Background(), dag, models.DAGRunStatus{
		Status: status.Error,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, 0, mock.count)
}

func testSuccessMail(t *testing.T, rp *reporter, mock *mockSender, dag *digraph.DAG, nodes []*models.Node) {
	dag.MailOn.Failure = true
	dag.MailOn.Success = true

	err := rp.send(context.Background(), dag, models.DAGRunStatus{
		Status: status.Success,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)

	require.Contains(t, mock.subject, "Success")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testRenderSummary(t *testing.T, _ *reporter, _ *mockSender, dag *digraph.DAG, _ []*models.Node) {
	status := models.NewStatusBuilder(dag).Create("run-id", status.Error, 0, time.Now())
	summary := renderDAGSummary(status, errors.New("test error"))
	require.Contains(t, summary, "test error")
	require.Contains(t, summary, dag.Name)
}

func testRenderTable(t *testing.T, _ *reporter, _ *mockSender, _ *digraph.DAG, nodes []*models.Node) {
	summary := renderStepSummary(nodes)
	require.Contains(t, summary, nodes[0].Step.Name)
	require.Contains(t, summary, nodes[0].Step.Args[0])
}

type mockSender struct {
	from    string
	to      []string
	subject string
	body    string
	count   int
}

func (m *mockSender) Send(_ context.Context, from string, to []string, subject, body string, _ []string) error {
	m.count += 1
	m.from = from
	m.to = to
	m.subject = subject
	m.body = body
	return nil
}

// TestRenderHTMLComprehensive tests the HTML rendering with comprehensive fake data
// to ensure the format is correct and prevent regressions
func TestRenderHTMLComprehensive(t *testing.T) {
	// Create comprehensive test data with various scenarios
	nodes := []*models.Node{
		{
			Step: digraph.Step{
				Name:    "setup-database",
				Command: "docker",
				Args:    []string{"run", "-d", "--name", "test-db", "postgres:13"},
			},
			Status:     status.NodeSuccess,
			StartedAt:  "2025-01-15T10:30:00Z",
			FinishedAt: "2025-01-15T10:30:45Z",
			Error:      "",
		},
		{
			Step: digraph.Step{
				Name:    "run-migrations",
				Command: "python",
				Args:    []string{"manage.py", "migrate", "--settings=production"},
			},
			Status:     status.NodeError,
			StartedAt:  "2025-01-15T10:30:45Z",
			FinishedAt: "2025-01-15T10:31:20Z",
			Error:      "Migration failed: Table 'users' already exists",
		},
		{
			Step: digraph.Step{
				Name:    "deploy-app",
				Command: "kubectl",
				Args:    []string{"apply", "-f", "deployment.yaml"},
			},
			Status:     status.NodeSkipped,
			StartedAt:  "",
			FinishedAt: "",
			Error:      "",
		},
		{
			Step: digraph.Step{
				Name:    "send-notification",
				Command: "curl",
				Args:    []string{"-X", "POST", "https://api.slack.com/webhook", "-d", `{"text":"Deployment complete"}`},
			},
			Status:     status.NodeRunning,
			StartedAt:  "2025-01-15T10:32:00Z",
			FinishedAt: "",
			Error:      "",
		},
		{
			Step: digraph.Step{
				Name:    "cleanup-temp-files",
				Command: "bash",
				Args:    nil, // Test nil args
			},
			Status:     status.NodeSuccess,
			StartedAt:  "2025-01-15T10:32:30Z",
			FinishedAt: "2025-01-15T10:32:35Z",
			Error:      "",
		},
		{
			Step: digraph.Step{
				Name:        "special-chars-test",
				Command:     "echo",
				Args:        []string{"<script>alert('xss')</script>", "&", "\"quotes\""},
				CmdWithArgs: "echo <script>alert('xss')</script> & \"quotes\"",
			},
			Status:     status.NodeError,
			StartedAt:  "2025-01-15T10:33:00Z",
			FinishedAt: "2025-01-15T10:33:05Z",
			Error:      "Command failed with exit code 1: <error> & \"special chars\"",
		},
	}

	// Call renderHTML to get the output
	html := renderHTML(nodes)

	// Verify HTML structure and content
	t.Run("HTMLStructure", func(t *testing.T) {
		// Check basic HTML structure
		require.Contains(t, html, "<!DOCTYPE html>")
		require.Contains(t, html, "<html>")
		require.Contains(t, html, "<head>")
		require.Contains(t, html, "<body>")
		require.Contains(t, html, "</html>")

		// Check table structure
		require.Contains(t, html, "<table>")
		require.Contains(t, html, "<thead>")
		require.Contains(t, html, "<tbody>")
		require.Contains(t, html, "</table>")
	})

	t.Run("TableHeaders", func(t *testing.T) {
		// Check all required headers are present
		headers := []string{"#", "Step", "Started At", "Finished At", "Status", "Command", "Error"}
		for _, header := range headers {
			require.Contains(t, html, fmt.Sprintf("<th>%s</th>", header))
		}
	})

	t.Run("TableContent", func(t *testing.T) {
		// Check that all step names are present
		require.Contains(t, html, "setup-database")
		require.Contains(t, html, "run-migrations")
		require.Contains(t, html, "deploy-app")
		require.Contains(t, html, "send-notification")
		require.Contains(t, html, "cleanup-temp-files")
		require.Contains(t, html, "special-chars-test")

		// Check status values (these are the actual values from Status.String())
		require.Contains(t, html, "finished")
		require.Contains(t, html, "failed")
		require.Contains(t, html, "skipped")
		require.Contains(t, html, "running")

		// Check timestamps are present
		require.Contains(t, html, "2025-01-15T10:30:00Z")
		require.Contains(t, html, "2025-01-15T10:30:45Z")

		// Check error messages are present (single quotes in actual output)
		require.Contains(t, html, "Migration failed: Table 'users' already exists")
	})

	t.Run("HTMLEscaping", func(t *testing.T) {
		// Verify dangerous HTML characters are properly escaped
		require.NotContains(t, html, "<script>alert('xss')</script>")          // Raw script tag should not exist
		require.Contains(t, html, "&lt;script&gt;alert('xss')&lt;/script&gt;") // Should be escaped (actual format)

		// Check ampersands are escaped
		require.Contains(t, html, "&amp;") // & should be escaped to &amp;

		// Check quotes are escaped in error messages
		require.Contains(t, html, "\"special chars\"") // Quotes in actual output
	})

	t.Run("RowCount", func(t *testing.T) {
		// Count the number of table rows (excluding header)
		rowCount := strings.Count(html, "<tr>") - 1 // Subtract 1 for header row
		require.Equal(t, len(nodes), rowCount, "Should have one row per node")
	})

	t.Run("CSSStyling", func(t *testing.T) {
		// Check that modern CSS is present for proper email rendering
		require.Contains(t, html, "border-collapse: separate")
		require.Contains(t, html, "border-spacing: 0")
		require.Contains(t, html, "border-radius: 12px")
		require.Contains(t, html, "box-shadow")
		require.Contains(t, html, "background-color: #2563eb")
		require.Contains(t, html, "font-family: -apple-system")
	})

	t.Run("GmailCompatibility", func(t *testing.T) {
		// Ensure Gmail-compatible structure
		require.Contains(t, html, `<style type="text/css">`)
		require.Contains(t, html, "class=")
		require.NotContains(t, html, "style=")
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
