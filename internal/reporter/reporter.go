package reporter

import (
	"bytes"
	"fmt"
	"jobctl/internal/config"
	"jobctl/internal/mail"
	"jobctl/internal/scheduler"
	"jobctl/internal/utils"
	"log"
	"strings"
)

type Reporter struct {
	*Config
}

type Config struct {
	Mailer mail.Mailer
}

func New(config *Config) *Reporter {
	return &Reporter{
		Config: config,
	}
}

func (rp *Reporter) ReportStep(sc *scheduler.Scheduler, g *scheduler.ExecutionGraph,
	cfg *config.Config, node *scheduler.Node) error {
	status := node.ReadStatus()
	if status != scheduler.NodeStatus_None {
		log.Printf("%s %s", node.Name, status)
	}
	if status == scheduler.NodeStatus_Error && node.MailOnError {
		return rp.sendError(cfg, sc.Status(g), g.Nodes())
	}
	return nil
}

func (rp *Reporter) Report(status scheduler.SchedulerStatus,
	nodes []*scheduler.Node, err error) {
	log.Printf(toText(status, nodes, err))
}

func (rp *Reporter) ReportMail(status scheduler.SchedulerStatus,
	g *scheduler.ExecutionGraph, err error, cfg *config.Config) error {
	if err != nil && status != scheduler.SchedulerStatus_Cancel && cfg.MailOn.Failure {
		return rp.sendError(cfg, status, g.Nodes())
	} else if cfg.MailOn.Success {
		return rp.sendInfo(cfg, status, g.Nodes())
	}
	return nil
}

func (rp *Reporter) sendInfo(cfg *config.Config,
	status scheduler.SchedulerStatus, nodes []*scheduler.Node) error {
	mailConfig := cfg.InfoMail
	jobName := cfg.Name
	subject := fmt.Sprintf("%s %s (%s)", mailConfig.Prefix, jobName, status)
	body := toHtml(status, nodes)

	return rp.Mailer.SendMail(
		cfg.InfoMail.From,
		[]string{cfg.InfoMail.To},
		subject,
		body,
	)
}

func (rp *Reporter) sendError(cfg *config.Config,
	status scheduler.SchedulerStatus, nodes []*scheduler.Node) error {
	mailConfig := cfg.ErrorMail
	jobName := cfg.Name
	subject := fmt.Sprintf("%s %s (%s)", mailConfig.Prefix, jobName, status)
	body := toHtml(status, nodes)

	return rp.Mailer.SendMail(
		cfg.ErrorMail.From,
		[]string{cfg.ErrorMail.To},
		subject,
		body,
	)
}

func toText(status scheduler.SchedulerStatus, nodes []*scheduler.Node, err error) string {
	vals := []string{}
	vals = append(vals, "[Result]")
	for _, n := range nodes {
		vals = append(vals, fmt.Sprintf("\t%s", n.Report()))
	}
	if err != nil {
		vals = append(vals, fmt.Sprintf("\tLast Error=%s", err.Error()))
	}
	return strings.Join(vals, "\n")
}

func toHtml(status scheduler.SchedulerStatus, list []*scheduler.Node) string {
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
	for _, n := range list {
		buffer.WriteString("<tr>")
		addValFunc(n.Name)
		addValFunc(utils.FormatTime(n.StartedAt))
		addValFunc(utils.FormatTime(n.FinishedAt))
		addStatusFunc(n.ReadStatus())
		if n.Error != nil {
			addValFunc(n.Error.Error())
		} else {
			addValFunc("-")
		}
		buffer.WriteString("</tr>")
	}
	buffer.WriteString("</table>")
	return buffer.String()
}
