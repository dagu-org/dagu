package reporter

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
)

// Reporter is responsible for reporting the status of the scheduler
// to the user.
type Reporter struct {
	*Config
}

// Config is the configuration for the reporter.
type Config struct {
	Mailer Mailer
}

// Mailer is a mailer interface.
type Mailer interface {
	SendMail(from string, to []string, subject, body string) error
}

// ReportStep is a function that reports the status of a step.
func (rp *Reporter) ReportStep(cfg *config.DAG, status *models.Status, node *scheduler.Node) error {
	st := node.ReadStatus()
	if st != scheduler.NodeStatus_None {
		log.Printf("%s %s", node.Name, status.StatusText)
	}
	if st == scheduler.NodeStatus_Error && node.MailOnError {
		return rp.Mailer.SendMail(
			cfg.ErrorMail.From,
			[]string{cfg.ErrorMail.To},
			fmt.Sprintf("%s %s (%s)", cfg.ErrorMail.Prefix, cfg.Name, status.Status),
			renderHTML(status.Nodes),
		)
	}
	return nil
}

// ReportSummary is a function that reports the status of the scheduler.
func (rp *Reporter) ReportSummary(status *models.Status, err error) {
	var buf bytes.Buffer
	buf.Write([]byte("\n"))
	buf.Write([]byte("Summary ->\n"))
	buf.Write([]byte(renderSummary(status, err)))
	buf.Write([]byte("\n"))
	buf.Write([]byte("Details ->\n"))
	buf.Write([]byte(renderTable(status.Nodes)))
	log.Print(buf.String())
}

// SendMail is a function that sends a report mail.
func (rp *Reporter) SendMail(cfg *config.DAG, status *models.Status, err error) error {
	if err != nil || status.Status == scheduler.SchedulerStatus_Error {
		if cfg.MailOn != nil && cfg.MailOn.Failure {
			return rp.Mailer.SendMail(
				cfg.ErrorMail.From,
				[]string{cfg.ErrorMail.To},
				fmt.Sprintf("%s %s (%s)", cfg.ErrorMail.Prefix, cfg.Name, status.Status),
				renderHTML(status.Nodes),
			)
		}
	} else if status.Status == scheduler.SchedulerStatus_Success {
		if cfg.MailOn != nil && cfg.MailOn.Success {
			rp.Mailer.SendMail(
				cfg.InfoMail.From,
				[]string{cfg.InfoMail.To},
				fmt.Sprintf("%s %s (%s)", cfg.InfoMail.Prefix, cfg.Name, status.Status),
				renderHTML(status.Nodes),
			)
		}
	}
	return nil
}

func renderSummary(status *models.Status, err error) string {
	t := table.NewWriter()
	var errText = ""
	if err != nil {
		errText = err.Error()
	}
	t.AppendHeader(table.Row{"RequestID", "Name", "Started At", "Finished At", "Status", "Params", "Error"})
	t.AppendRow(table.Row{
		status.RequestId,
		status.Name,
		status.StartedAt,
		status.FinishedAt,
		status.Status,
		status.Params,
		errText,
	})
	return t.Render()
}

func renderTable(nodes []*models.Node) string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Step", "Started At", "Finished At", "Status", "Command", "Error"})
	for i, n := range nodes {
		var command = n.Command
		if n.Args != nil {
			command = strings.Join([]string{n.Command, strings.Join(n.Args, " ")}, " ")
		}
		t.AppendRow(table.Row{
			fmt.Sprintf("%d", i+1),
			n.Name,
			n.StartedAt,
			n.FinishedAt,
			n.StatusText,
			command,
			n.Error,
		})
	}
	return t.Render()
}

func renderHTML(nodes []*models.Node) string {
	var buffer bytes.Buffer
	addValFunc := func(val string) {
		buffer.WriteString(
			fmt.Sprintf("<td align=\"center\" style=\"padding: 10px;\">%s</td>",
				val))
	}
	buffer.WriteString(`
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
		style := ""
		switch status {
		case scheduler.NodeStatus_Error:
			style = "color: #D01117;font-weight:bold;"
		}
		buffer.WriteString(
			fmt.Sprintf("<td align=\"center\" style=\"padding: 10px; %s\">%s</td>",
				style, status))
	}
	for _, n := range nodes {
		buffer.WriteString("<tr>")
		addValFunc(n.Name)
		addValFunc(n.StartedAt)
		addValFunc(n.FinishedAt)
		addStatusFunc(n.Status)
		addValFunc(n.Error)
		buffer.WriteString("</tr>")
	}
	buffer.WriteString("</table>")
	return buffer.String()
}
