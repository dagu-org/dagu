// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/jedib0t/go-pretty/v6/table"
)

// Sender is a mailer interface.
type Sender interface {
	Send(
		from string, to []string, subject, body string, attachments []string,
	) error
}

// reporter is responsible for reporting the status of the scheduler
// to the user.
type reporter struct {
	sender Sender
	logger logger.Logger
}

func newReporter(sender Sender, lg logger.Logger) *reporter {
	return &reporter{
		sender: sender,
		logger: lg,
	}
}

// reportStep is a function that reports the status of a step.
func (r *reporter) reportStep(
	workflow *digraph.DAG, status *model.Status, node *scheduler.Node,
) error {
	nodeStatus := node.State().Status
	if nodeStatus != scheduler.NodeStatusNone {
		r.logger.Info("Step execution finished",
			"step", node.Data().Step.Name,
			"status", nodeStatus,
		)
	}
	if nodeStatus == scheduler.NodeStatusError && node.Data().Step.MailOnError {
		return r.sender.Send(
			workflow.ErrorMail.From,
			[]string{workflow.ErrorMail.To},
			fmt.Sprintf(
				"%s %s (%s)", workflow.ErrorMail.Prefix, workflow.Name, status.Status,
			),
			renderHTML(status.Nodes),
			addAttachmentList(workflow.ErrorMail.AttachLogs, status.Nodes),
		)
	}
	return nil
}

// report is a function that reports the status of the scheduler.
func (r *reporter) report(status *model.Status, err error) {
	var buf bytes.Buffer
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte("Summary ->\n"))
	_, _ = buf.Write([]byte(renderSummary(status, err)))
	_, _ = buf.Write([]byte("\n"))
	_, _ = buf.Write([]byte("Details ->\n"))
	_, _ = buf.Write([]byte(renderTable(status.Nodes)))
	r.logger.Write(buf.String())
}

// send is a function that sends a report mail.
func (r *reporter) send(
	workflow *digraph.DAG, status *model.Status, err error,
) error {
	if err != nil || status.Status == scheduler.StatusError {
		if workflow.MailOn != nil && workflow.MailOn.Failure {
			return r.sender.Send(
				workflow.ErrorMail.From,
				[]string{workflow.ErrorMail.To},
				fmt.Sprintf(
					"%s %s (%s)", workflow.ErrorMail.Prefix, workflow.Name, status.Status,
				),
				renderHTML(status.Nodes),
				addAttachmentList(workflow.ErrorMail.AttachLogs, status.Nodes),
			)
		}
	} else if status.Status == scheduler.StatusSuccess {
		if workflow.MailOn != nil && workflow.MailOn.Success {
			_ = r.sender.Send(
				workflow.InfoMail.From,
				[]string{workflow.InfoMail.To},
				fmt.Sprintf(
					"%s %s (%s)", workflow.InfoMail.Prefix, workflow.Name, status.Status,
				),
				renderHTML(status.Nodes),
				addAttachmentList(workflow.InfoMail.AttachLogs, status.Nodes),
			)
		}
	}
	return nil
}

func renderSummary(status *model.Status, err error) string {
	t := table.NewWriter()
	var errText string
	if err != nil {
		errText = err.Error()
	}
	t.AppendHeader(
		table.Row{
			"RequestID",
			"Name",
			"Started At",
			"Finished At",
			"Status",
			"Params",
			"Error",
		},
	)
	t.AppendRow(table.Row{
		status.RequestID,
		status.Name,
		status.StartedAt,
		status.FinishedAt,
		status.Status,
		status.Params,
		errText,
	})
	return t.Render()
}

func renderTable(nodes []*model.Node) string {
	t := table.NewWriter()
	t.AppendHeader(
		table.Row{
			"#",
			"Step",
			"Started At",
			"Finished At",
			"Status",
			"Command",
			"Error",
		},
	)
	for i, n := range nodes {
		var command = n.Step.Command
		if n.Step.Args != nil {
			command = strings.Join(
				[]string{n.Step.Command, strings.Join(n.Step.Args, " ")}, " ",
			)
		}
		t.AppendRow(table.Row{
			fmt.Sprintf("%d", i+1),
			n.Step.Name,
			n.StartedAt,
			n.FinishedAt,
			n.StatusText,
			command,
			n.Error,
		})
	}
	return t.Render()
}

func renderHTML(nodes []*model.Node) string {
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

func addAttachmentList(
	trigger bool, nodes []*model.Node,
) (attachments []string) {
	if trigger {
		for _, n := range nodes {
			attachments = append(attachments, n.Log)
		}
	}
	return attachments
}
