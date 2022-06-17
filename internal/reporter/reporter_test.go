package reporter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

func TestReporter(t *testing.T) {

	for scenario, fn := range map[string]func(
		t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node,
	){
		"create errormail":   testErrorMail,
		"no errormail":       testNoErrorMail,
		"create successmail": testSuccessMail,
		"create summary":     testRenderSummary,
		"create node list":   testRenderTable,
		"report summary":     testReportSummary,
		"report step":        testReportStep,
	} {
		t.Run(scenario, func(t *testing.T) {

			cfg := &config.Config{
				Name: "test DAG",
				MailOn: config.MailOn{
					Failure: true,
				},
				ErrorMail: &config.MailConfig{
					Prefix: "Error: ",
					From:   "from@mailer.com",
					To:     "to@mailer.com",
				},
				InfoMail: &config.MailConfig{
					Prefix: "Success: ",
					From:   "from@mailer.com",
					To:     "to@mailer.com",
				},
				Steps: []*config.Step{
					{
						Name:    "test-step",
						Command: "true",
					},
				},
			}

			nodes := []*models.Node{
				{
					Step: &config.Step{
						Name:    "test-step",
						Command: "true",
						Args:    []string{"param-x"},
					},
					Status:     scheduler.NodeStatus_Running,
					StartedAt:  utils.FormatTime(time.Now()),
					FinishedAt: utils.FormatTime(time.Now().Add(time.Minute * 10)),
				},
			}

			rp := &Reporter{
				Config: &Config{
					Mailer: &mockMailer{},
				},
			}

			fn(t, rp, cfg, nodes)
		})
	}
}

func testErrorMail(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	cfg.MailOn.Failure = true
	cfg.MailOn.Success = false

	rp.ReportMail(cfg, &models.Status{
		Status: scheduler.SchedulerStatus_Error,
		Nodes:  nodes,
	}, fmt.Errorf("Error"))

	mock := rp.Mailer.(*mockMailer)
	require.Contains(t, mock.subject, "Error")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testNoErrorMail(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	cfg.MailOn.Failure = false
	cfg.MailOn.Success = true

	rp.ReportMail(cfg, &models.Status{
		Status: scheduler.SchedulerStatus_Error,
		Nodes:  nodes,
	}, nil)

	mock := rp.Mailer.(*mockMailer)
	require.Equal(t, 0, mock.count)
}

func testSuccessMail(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	cfg.MailOn.Failure = true
	cfg.MailOn.Success = true

	rp.ReportMail(cfg, &models.Status{
		Status: scheduler.SchedulerStatus_Success,
		Nodes:  nodes,
	}, nil)

	mock := rp.Mailer.(*mockMailer)
	require.Contains(t, mock.subject, "Success")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testReportSummary(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	rp.ReportSummary(&models.Status{
		Status: scheduler.SchedulerStatus_Success,
		Nodes:  nodes,
	}, errors.New("test error"))

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, "test error")
}

func testReportStep(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	cfg.Steps[0].MailOnError = true
	rp.ReportStep(
		cfg,
		&models.Status{
			Status: scheduler.SchedulerStatus_Running,
			Nodes:  nodes,
		},
		&scheduler.Node{
			Step: cfg.Steps[0],
			NodeState: scheduler.NodeState{
				Status: scheduler.NodeStatus_Error,
			},
		},
	)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, cfg.Steps[0].Name)

	mock := rp.Mailer.(*mockMailer)
	require.Equal(t, 1, mock.count)
}

func testRenderSummary(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	status := &models.Status{
		Name:   cfg.Name,
		Status: scheduler.SchedulerStatus_Error,
		Nodes:  nodes,
	}
	summary := renderSummary(status, errors.New("test error"))
	require.Contains(t, summary, "test error")
	require.Contains(t, summary, cfg.Name)
}

func testRenderTable(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	summary := renderTable(nodes)
	require.Contains(t, summary, nodes[0].Name)
	require.Contains(t, summary, nodes[0].Args[0])
}

type mockMailer struct {
	from    string
	to      []string
	subject string
	body    string
	count   int
}

var _ Mailer = (*mockMailer)(nil)

func (m *mockMailer) SendMail(from string, to []string, subject, body string) error {
	m.count += 1
	m.from = from
	m.to = to
	m.subject = subject
	m.body = body
	return nil
}
