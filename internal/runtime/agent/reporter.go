package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/runtime"
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
	ctx context.Context, dag *core.DAG, dagStatus models.DAGRunStatus, node *runtime.Node,
) error {
	nodeStatus := node.State().Status
	if nodeStatus != status.NodeNone {
		logger.Info(ctx, "Step finished", "step", node.NodeData().Step.Name, "status", nodeStatus)
	}
	if nodeStatus == status.NodeError && node.NodeData().Step.MailOnError && dag.ErrorMail != nil {
		fromAddress := dag.ErrorMail.From
		toAddresses := dag.ErrorMail.To
		subject := fmt.Sprintf("%s %s (%s)", dag.ErrorMail.Prefix, dag.Name, dagStatus.Status)
		html := renderHTMLWithDAGInfo(dagStatus)
		attachments := addAttachments(dag.ErrorMail.AttachLogs, dagStatus.Nodes)
		return r.senderFn(ctx, fromAddress, toAddresses, subject, html, attachments)
	}
	return nil
}

// report is a function that reports the status of the scheduler.
func (r *reporter) getSummary(_ context.Context, dagStatus models.DAGRunStatus, err error) string {
	var buf bytes.Buffer
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte("Summary ->\n"))
	_, _ = buf.Write([]byte(renderDAGSummary(dagStatus, err)))
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte("Details ->\n"))
	_, _ = buf.Write([]byte(renderStepSummary(dagStatus.Nodes)))
	return buf.String()
}

// send is a function that sends a report mail.
func (r *reporter) send(ctx context.Context, dag *core.DAG, dagStatus models.DAGRunStatus, err error) error {
	if err != nil || dagStatus.Status == status.Error {
		if dag.MailOn != nil && dag.MailOn.Failure && dag.ErrorMail != nil {
			fromAddress := dag.ErrorMail.From
			toAddresses := dag.ErrorMail.To
			subject := fmt.Sprintf("%s %s (%s)", dag.ErrorMail.Prefix, dag.Name, dagStatus.Status)
			html := renderHTMLWithDAGInfo(dagStatus)
			attachments := addAttachments(dag.ErrorMail.AttachLogs, dagStatus.Nodes)
			return r.senderFn(ctx, fromAddress, toAddresses, subject, html, attachments)
		}
	} else if dagStatus.Status == status.Success || dagStatus.Status == status.PartialSuccess {
		if dag.MailOn != nil && dag.MailOn.Success && dag.InfoMail != nil {
			fromAddress := dag.InfoMail.From
			toAddresses := dag.InfoMail.To
			subject := fmt.Sprintf("%s %s (%s)", dag.InfoMail.Prefix, dag.Name, dagStatus.Status)
			html := renderHTMLWithDAGInfo(dagStatus)
			attachments := addAttachments(dag.InfoMail.AttachLogs, dagStatus.Nodes)
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

func renderDAGSummary(dagStatus models.DAGRunStatus, err error) string {
	dataRow := table.Row{
		dagStatus.DAGRunID,
		dagStatus.Name,
		dagStatus.StartedAt,
		dagStatus.FinishedAt,
		dagStatus.Status,
		dagStatus.Params,
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
        .status-partial-success { color: #ea580c; font-weight: 500; }
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
		case "partial success":
			statusClass = "status-partial-success"
		}
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"%s\">%s</td>", statusClass, status))

		// Command (join args and escape HTML)
		command := n.Step.CmdWithArgs
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

func renderHTMLWithDAGInfo(dagStatus models.DAGRunStatus) string {
	var buffer bytes.Buffer

	// Start with enhanced HTML structure and styling
	_, _ = buffer.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style type="text/css">
        body { 
            margin: 0; 
            padding: 20px; 
            background-color: #f8fafc; 
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #334155;
        }
        .container { 
            max-width: 1000px; 
            margin: 0 auto; 
        }
        .dag-info {
            background: linear-gradient(135deg, #f8fafc 0%, #ffffff 100%);
            border-radius: 16px;
            padding: 32px;
            margin-bottom: 32px;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
            border: 1px solid #e2e8f0;
            position: relative;
            overflow: hidden;
        }
        .dag-info::before {
            content: "";
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 4px;
            background: linear-gradient(90deg, #3b82f6 0%, #2563eb 50%, #1d4ed8 100%);
        }
        .dag-info h2 {
            margin: 32px 0 24px 0;
            color: #1e293b;
            font-size: 24px;
            font-weight: 700;
            letter-spacing: -0.5px;
        }
        .status-badge {
            position: absolute;
            top: 24px;
            right: 24px;
            display: inline-flex;
            align-items: center;
            padding: 8px 16px;
            border-radius: 24px;
            font-size: 13px;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .status-badge.success {
            background-color: #d1fae5;
            color: #065f46;
            box-shadow: 0 0 0 1px rgba(16, 185, 129, 0.1);
        }
        .status-badge.failed {
            background-color: #fee2e2;
            color: #991b1b;
            box-shadow: 0 0 0 1px rgba(239, 68, 68, 0.1);
        }
        .status-badge.running {
            background-color: #dbeafe;
            color: #1e40af;
            box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.1);
        }
        .status-badge.skipped {
            background-color: #f3f4f6;
            color: #374151;
            box-shadow: 0 0 0 1px rgba(107, 114, 128, 0.1);
        }
        .dag-info-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 24px;
        }
        .info-item {
            background-color: #ffffff;
            padding: 16px;
            border-radius: 8px;
            border: 1px solid #e5e7eb;
        }
        .dag-info-label {
            font-weight: 600;
            color: #6b7280;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 4px;
        }
        .dag-info-value {
            color: #1e293b;
            font-size: 16px;
            font-weight: 500;
        }
        .dag-info-value.mono {
            font-family: "SF Mono", "Consolas", "Monaco", monospace;
            font-size: 14px;
            color: #0f172a;
            background-color: #f1f5f9;
            padding: 6px 10px;
            border-radius: 4px;
            display: inline-block;
            margin-top: 4px;
            word-break: break-all;
        }
        .dag-name {
            color: #1e293b;
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 4px;
        }
        table { 
            border-collapse: separate; 
            border-spacing: 0; 
            width: 100%; 
            background-color: #ffffff; 
            border-radius: 12px; 
            overflow: hidden;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
            border: 1px solid #e2e8f0;
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
            border-bottom: 1px solid #f1f5f9; 
            vertical-align: top;
            font-size: 13px;
            line-height: 1.5;
        }
        tr:hover { background-color: #f8fafc; }
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
        .command-cell { font-family: "SF Mono", "Consolas", "Monaco", monospace; font-size: 12px; color: #475569; }
        .error-cell { color: #dc2626; font-size: 12px; }
        .timestamp { color: #64748b; font-size: 12px; }
    </style>
</head>
<body>
<div class="container">
    <div class="dag-info">
        <div class="status-badge `)

	// Add status class
	statusStr := dagStatus.Status.String()
	statusClass := ""
	switch statusStr {
	case "finished":
		statusClass = "success"
	case "failed":
		statusClass = "failed"
	case "running":
		statusClass = "running"
	case "skipped":
		statusClass = "skipped"
	}
	_, _ = buffer.WriteString(statusClass)
	_, _ = buffer.WriteString(`">`)
	_, _ = buffer.WriteString(strings.ToUpper(statusStr))
	_, _ = buffer.WriteString(`</div>
        <h2>DAG Execution Details</h2>
        <div class="dag-name">`)

	// Add DAG name (escaped)
	dagName := strings.ReplaceAll(dagStatus.Name, "&", "&amp;")
	dagName = strings.ReplaceAll(dagName, "<", "&lt;")
	dagName = strings.ReplaceAll(dagName, ">", "&gt;")
	_, _ = buffer.WriteString(dagName)

	_, _ = buffer.WriteString(`</div>
        <div class="dag-info-grid">
            <div class="info-item">
                <div class="dag-info-label">DAG Run ID</div>
                <div class="dag-info-value mono">`)

	// Add DAG Run ID (escaped)
	dagRunID := strings.ReplaceAll(dagStatus.DAGRunID, "&", "&amp;")
	dagRunID = strings.ReplaceAll(dagRunID, "<", "&lt;")
	dagRunID = strings.ReplaceAll(dagRunID, ">", "&gt;")
	_, _ = buffer.WriteString(dagRunID)

	_, _ = buffer.WriteString(`</div>
            </div>
            <div class="info-item">
                <div class="dag-info-label">Parameters</div>
                <div class="dag-info-value mono">`)

	// Add Parameters (escaped)
	params := dagStatus.Params
	if params == "" {
		params = "(none)"
	}
	params = strings.ReplaceAll(params, "&", "&amp;")
	params = strings.ReplaceAll(params, "<", "&lt;")
	params = strings.ReplaceAll(params, ">", "&gt;")
	params = strings.ReplaceAll(params, "\"", "&quot;")
	_, _ = buffer.WriteString(params)

	_, _ = buffer.WriteString(`</div>
            </div>
            <div class="info-item">
                <div class="dag-info-label">Started At</div>
                <div class="dag-info-value">`)
	_, _ = buffer.WriteString(dagStatus.StartedAt)

	_, _ = buffer.WriteString(`</div>
            </div>
            <div class="info-item">
                <div class="dag-info-label">Finished At</div>
                <div class="dag-info-value">`)
	_, _ = buffer.WriteString(dagStatus.FinishedAt)

	_, _ = buffer.WriteString(`</div>
            </div>
        </div>
    </div>
    
    <table>
    <thead>
    <tr>`)

	// Add table headers
	headers := []string{"#", "Step", "Started At", "Finished At", "Status", "Command", "Error"}
	for _, header := range headers {
		_, _ = buffer.WriteString(fmt.Sprintf("<th>%s</th>", header))
	}
	_, _ = buffer.WriteString("</tr></thead><tbody>")

	// Add table rows (reuse the logic from renderHTML)
	for i, n := range dagStatus.Nodes {
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
		nodeStatus := n.Status.String()
		nodeStatusClass := ""
		switch nodeStatus {
		case "finished":
			nodeStatusClass = "status-finished"
		case "failed":
			nodeStatusClass = "status-failed"
		case "running":
			nodeStatusClass = "status-running"
		case "skipped":
			nodeStatusClass = "status-skipped"
		}
		_, _ = buffer.WriteString(fmt.Sprintf("<td class=\"%s\">%s</td>", nodeStatusClass, nodeStatus))

		// Command (join args and escape HTML)
		command := n.Step.CmdWithArgs
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

	_, _ = buffer.WriteString("</tbody></table></div></body></html>")

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
