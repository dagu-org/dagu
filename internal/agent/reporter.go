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
	ctx context.Context, dag *digraph.DAG, status models.Status, node *scheduler.Node,
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
func (r *reporter) getSummary(_ context.Context, status models.Status, err error) string {
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
func (r *reporter) send(ctx context.Context, dag *digraph.DAG, status models.Status, err error) error {
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
	"ExecID",
	"Name",
	"Started At",
	"Finished At",
	"Status",
	"Params",
	"Error",
}

func renderDAGSummary(status models.Status, err error) string {
	dataRow := table.Row{
		status.ExecID,
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
	addValFunc := func(val string) {
		_, _ = buffer.WriteString(
			fmt.Sprintf(
				"<td align=\"center\" style=\"padding: 10px;\">%s</td>",
				val,
			))
	}
	_, _ = buffer.WriteString(`
	<table border="1" style="border-collapse: collapse;">
		<thead>
			<tr>
				<th align="center" style="padding: 10px;">Name</th>
				<th align="center" style="padding: 10px;">Started At</th>
				<th align="center" style="padding: 10px;">Finished At</th>
				<th align="center" style="padding: 10px;">Status</th>
				<th align="center" style="padding: 10px;">Error</th>
			</tr>
		</thead>
		<tbody>
	`)
	addStatusFunc := func(status scheduler.NodeStatus) {
		var style string
		if status == scheduler.NodeStatusError {
			style = "color: #D01117;font-weight:bold;"
		}

		_, _ = buffer.WriteString(
			fmt.Sprintf(
				"<td align=\"center\" style=\"padding: 10px; %s\">%s</td>",
				style, status,
			))
	}
	for _, n := range nodes {
		_, _ = buffer.WriteString("<tr>")
		addValFunc(n.Step.Name)
		addValFunc(n.StartedAt)
		addValFunc(n.FinishedAt)
		addStatusFunc(n.Status)
		addValFunc(n.Error)
		_, _ = buffer.WriteString("</tr>")
	}
	_, _ = buffer.WriteString("</table>")
	return buffer.String()
}

func addAttachments(
	trigger bool, nodes []*models.Node,
) (attachments []string) {
	if trigger {
		for _, n := range nodes {
			attachments = append(attachments, n.Log)
		}
	}
	return attachments
}
