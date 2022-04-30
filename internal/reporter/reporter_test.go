package reporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagman/internal/config"
	"github.com/yohamta/dagman/internal/mail"
	"github.com/yohamta/dagman/internal/models"
	"github.com/yohamta/dagman/internal/scheduler"
	"github.com/yohamta/dagman/internal/utils"
)

func TestReporter(t *testing.T) {

	for scenario, fn := range map[string]func(
		t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node,
	){
		"create errormail":   testErrorMail,
		"no errormail":       testNoErrorMail,
		"create successmail": testSuccessMail,
	} {
		t.Run(scenario, func(t *testing.T) {

			cfg := &config.Config{
				Name: "test DAG",
				MailOn: config.MailOn{
					Failure: true,
				},
				ErrorMail: &config.MailConfig{
					Prefix: "Error: ",
					From:   "from@mail.com",
					To:     "to@mail.com",
				},
				InfoMail: &config.MailConfig{
					Prefix: "Success: ",
					From:   "from@mail.com",
					To:     "to@mail.com",
				},
			}

			nodes := []*models.Node{
				{
					Step: &config.Step{
						Name:    "test-step",
						Command: "true",
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
	})

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
	})

	mock := rp.Mailer.(*mockMailer)
	require.Equal(t, 0, mock.count)
}

func testSuccessMail(t *testing.T, rp *Reporter, cfg *config.Config, nodes []*models.Node) {
	cfg.MailOn.Failure = true
	cfg.MailOn.Success = true

	rp.ReportMail(cfg, &models.Status{
		Status: scheduler.SchedulerStatus_Success,
		Nodes:  nodes,
	})

	mock := rp.Mailer.(*mockMailer)
	require.Contains(t, mock.subject, "Success")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

type mockMailer struct {
	from    string
	to      []string
	subject string
	body    string
	count   int
}

var _ mail.Mailer = (*mockMailer)(nil)

func (m *mockMailer) SendMail(from string, to []string, subject, body string) error {
	m.count += 1
	m.from = from
	m.to = to
	m.subject = subject
	m.body = body
	return nil
}
