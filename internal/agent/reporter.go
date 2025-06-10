package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/jedib0t/go-pretty/v6/table"
)

// Sender is a mailer interface.
type Sender interface {
	Send(ctx context.Context, from string, to []string, subject, body string, attachments []string) error
}

// SenderFn is a function type for sending reports.
type SenderFn func(ctx context.Context, from string, to []string, subject, body string, attachments []string) error

// reporter is responsible for reporting the status of the scheduler
// to the user.
type reporter struct{ senderFn SenderFn }

func newReporter(f SenderFn) *reporter {
	return &reporter{senderFn: f}
}

// reportStep is a function that reports the status of a step.
func (r *reporter) reportStep(
	ctx context.Context, dag *digraph.DAG, status models.DAGRunStatus, node *scheduler.Node,
) error {
	nodeStatus := node.State().Status
	if nodeStatus != scheduler.NodeStatusNone {
		logger.Info(ctx, "Step finished", "step", node.NodeData().Step.Name, "status", nodeStatus)
	}
	if nodeStatus == scheduler.NodeStatusError && node.NodeData().Step.MailOnError && dag.ErrorMail != nil {
		fromAddress := dag.ErrorMail.From
		toAddresses := []string{dag.ErrorMail.To}
		subject := fmt.Sprintf("%s %s (%s)", dag.ErrorMail.Prefix, dag.Name, status.Status)
		html := renderHTML(status.Nodes)
		attachments := addAttachments(dag.ErrorMail.AttachLogs, status.Nodes)
		return r.senderFn(ctx, fromAddress, toAddresses, subject, html, attachments)
	}
	return nil
}

// report is a function that reports the status of the scheduler.
func (r *reporter) getSummary(_ context.Context, status models.DAGRunStatus, err error) string {
	var buf bytes.Buffer
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte("Summary ->\n"))
	_, _ = buf.Write([]byte(renderDAGSummary(status, err)))
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte("Details ->\n"))
	_, _ = buf.Write([]byte(renderStepSummary(status.Nodes)))
	return buf.String()
}

// send is a function that sends a report mail.
func (r *reporter) send(ctx context.Context, dag *digraph.DAG, status models.DAGRunStatus, err error) error {
	if err != nil || status.Status == scheduler.StatusError {
		if dag.MailOn != nil && dag.MailOn.Failure && dag.ErrorMail != nil {
			fromAddress := dag.ErrorMail.From
			toAddresses := []string{dag.ErrorMail.To}
			subject := fmt.Sprintf("%s %s (%s)", dag.ErrorMail.Prefix, dag.Name, status.Status)
			html := renderHTML(status.Nodes)
			attachments := addAttachments(dag.ErrorMail.AttachLogs, status.Nodes)
			return r.senderFn(ctx, fromAddress, toAddresses, subject, html, attachments)
		}
	} else if status.Status == scheduler.StatusSuccess {
		if dag.MailOn != nil && dag.MailOn.Success && dag.InfoMail != nil {
			fromAddress := dag.InfoMail.From
			toAddresses := []string{dag.InfoMail.To}
			subject := fmt.Sprintf("%s %s (%s)", dag.InfoMail.Prefix, dag.Name, status.Status)
			html := renderHTML(status.Nodes)
			attachments := addAttachments(dag.InfoMail.AttachLogs, status.Nodes)
			_ = r.senderFn(ctx, fromAddress, toAddresses, subject, html, attachments)
		}
	}
	return nil
}

var dagHeader = table.Row{
	"Run ID",
	"Name",
	"Started At",
	"Finished At",
	"Status",
	"Params",
	"Error",
}

func renderDAGSummary(status models.DAGRunStatus, err error) string {
	dataRow := table.Row{
		status.DAGRunID,
		status.Name,
		status.StartedAt,
		status.FinishedAt,
		status.Status,
		status.Params,
	}
	if err != nil {
		dataRow = append(dataRow, err.Error())
	} else {
		dataRow = append(dataRow, "")
	}

	reportTable := table.NewWriter()
	reportTable.AppendHeader(dagHeader)
	reportTable.AppendRow(dataRow)
	return reportTable.Render()
}

var stepHeader = table.Row{
	"#",
	"Step",
	"Started At",
	"Finished At",
	"Status",
	"Command",
	"Error",
}

func renderStepSummary(nodes []*models.Node) string {
	stepTable := table.NewWriter()
	stepTable.AppendHeader(stepHeader)

	for i, n := range nodes {
		number := fmt.Sprintf("%d", i+1)
		dataRow := table.Row{
			number,
			n.Step.Name,
			n.StartedAt,
			n.FinishedAt,
			n.Status.String(),
		}
		if n.Step.Args != nil {
			dataRow = append(dataRow, strings.Join(n.Step.Args, " "))
		} else {
			dataRow = append(dataRow, "")
		}
		dataRow = append(dataRow, n.Error)
		stepTable.AppendRow(dataRow)
	}

	return stepTable.Render()
}

func renderHTML(nodes []*models.Node) string {
	var buffer bytes.Buffer

	// Start with basic HTML structure with improved styling
	_, _ = buffer.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style type="text/css">
        body { margin: 0; padding: 20px; background-color: #fafbfc; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
        table { 
            border-collapse: separate; 
            border-spacing: 0; 
            width: 100%; 
            max-width: 1200px;
            margin: 0 auto;
            background-color: #ffffff; 
            border-radius: 12px; 
            overflow: hidden;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
        }
        th { 
            background-color: #2563eb;
            color: #ffffff; 
            font-weight: 600; 
            padding: 16px 12px; 
            text-align: left;
            font-size: 14px;
            letter-spacing: 0.5px;
            text-transform: uppercase;
        }
        td { 
            padding: 14px 12px; 
            border-bottom: 1px solid #e8eef3; 
            vertical-align: top;
            font-size: 13px;
            line-height: 1.5;
        }
        tr:hover { background-color: #f8f9fb; }
        tr:last-child td { border-bottom: none; }
        .status-finished { color: #059669; font-weight: 500; }
        .status-failed { color: #dc2626; font-weight: 500; }
        .status-running { color: #2563eb; font-weight: 500; }
        .status-skipped { color: #6b7280; font-weight: 500; }
        .row-number { 
            background-color: #f1f5f9; 
            font-weight: 600; 
            text-align: center; 
            color: #475569;
            min-width: 40px;
        }
        .step-name { font-weight: 500; color: #1e293b; }
        .command-cell { font-family: "SF Mono", Monaco, "Cascadia Code", monospace; font-size: 12px; color: #475569; }
        .error-cell { color: #dc2626; font-size: 12px; }
        .timestamp { color: #64748b; font-size: 12px; }
    </style>
</head>
<body>
<table>
<thead>
<tr>`)

	// Add table headers
	headers := []string{"#", "Step", "Started At", "Finished At", "Status", "Command", "Error"}
	for _, header := range headers {
		_, _ = buffer.WriteString(fmt.Sprintf("<th>%s</th>", header))
	}
	_, _ = buffer.WriteString("</tr></thead><tbody>")

	// Add table rows
	for i, n := range nodes {
		_, _ = buffer.WriteString("<tr>")

		// Row number with special styling
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"row-number\">%d</td>", i+1))

		// Step name (escape HTML)
		stepName := strings.ReplaceAll(n.Step.Name, "&", "&amp;")
		stepName = strings.ReplaceAll(stepName, "<", "&lt;")
		stepName = strings.ReplaceAll(stepName, ">", "&gt;")
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"step-name\">%s</td>", stepName))

		// Started At with timestamp styling
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"timestamp\">%s</td>", n.StartedAt))

		// Finished At with timestamp styling
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"timestamp\">%s</td>", n.FinishedAt))

		// Status with conditional styling
		status := n.Status.String()
		statusClass := ""
		switch status {
		case "finished":
			statusClass = "status-finished"
		case "failed":
			statusClass = "status-failed"
		case "running":
			statusClass = "status-running"
		case "skipped":
			statusClass = "status-skipped"
		}
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"%s\">%s</td>", statusClass, status))

		// Command (join args and escape HTML)
		var command string
		if n.Step.Args != nil {
			command = strings.Join(n.Step.Args, " ")
		}
		command = strings.ReplaceAll(command, "&", "&amp;")
		command = strings.ReplaceAll(command, "<", "&lt;")
		command = strings.ReplaceAll(command, ">", "&gt;")
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"command-cell\">%s</td>", command))

		// Error (escape HTML)
		errorMsg := strings.ReplaceAll(n.Error, "&", "&amp;")
		errorMsg = strings.ReplaceAll(errorMsg, "<", "&lt;")
		errorMsg = strings.ReplaceAll(errorMsg, ">", "&gt;")
		if errorMsg != "" {
			_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"error-cell\">%s</td>", errorMsg))
		} else {
			_, _ = buffer.WriteString("<td></td>")
		}

		_, _ = buffer.WriteString("</tr>")
	}

	_, _ = buffer.WriteString("</tbody></table></body></html>")

	return buffer.String()
}

func addAttachments(
	trigger bool, nodes []*models.Node,
) (attachments []string) {
	if trigger {
		for _, n := range nodes {
			attachments = append(attachments, n.Stdout)
		}
	}
	return attachments
}
