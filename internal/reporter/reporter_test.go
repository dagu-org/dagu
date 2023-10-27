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

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestReporter(t *testing.T) {

	for scenario, fn := range map[string]func(
		t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node,
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

			d := &dag.DAG{
				Name: "test DAG",
				MailOn: &dag.MailOn{
					Failure: true,
				},
				ErrorMail: &dag.MailConfig{
					Prefix: "Error: ",
					From:   "from@mailer.com",
					To:     "to@mailer.com",
				},
				InfoMail: &dag.MailConfig{
					Prefix: "Success: ",
					From:   "from@mailer.com",
					To:     "to@mailer.com",
				},
				Steps: []*dag.Step{
					{
						Name:    "test-step",
						Command: "true",
					},
				},
			}

			nodes := []*model.Node{
				{
					Step: &dag.Step{
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

			fn(t, rp, d, nodes)
		})
	}
}

func testErrorMail(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
	d.MailOn.Failure = true
	d.MailOn.Success = false

	_ = rp.SendMail(d, &model.Status{
		Status: scheduler.SchedulerStatus_Error,
		Nodes:  nodes,
	}, fmt.Errorf("Error"))

	mock := rp.Mailer.(*mockMailer)
	require.Contains(t, mock.subject, "Error")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testNoErrorMail(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
	d.MailOn.Failure = false
	d.MailOn.Success = true

	err := rp.SendMail(d, &model.Status{
		Status: scheduler.SchedulerStatus_Error,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)

	mock := rp.Mailer.(*mockMailer)
	require.Equal(t, 0, mock.count)
}

func testSuccessMail(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
	d.MailOn.Failure = true
	d.MailOn.Success = true

	err := rp.SendMail(d, &model.Status{
		Status: scheduler.SchedulerStatus_Success,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)

	mock := rp.Mailer.(*mockMailer)
	require.Contains(t, mock.subject, "Success")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testReportSummary(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	rp.ReportSummary(&model.Status{
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

func testReportStep(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	d.Steps[0].MailOnError = true
	err = rp.ReportStep(
		d,
		&model.Status{
			Status: scheduler.SchedulerStatus_Running,
			Nodes:  nodes,
		},
		&scheduler.Node{
			Step: d.Steps[0],
			NodeState: scheduler.NodeState{
				Status: scheduler.NodeStatus_Error,
			},
		},
	)
	require.NoError(t, err)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, d.Steps[0].Name)

	mock := rp.Mailer.(*mockMailer)
	require.Equal(t, 1, mock.count)
}

func testRenderSummary(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
	status := &model.Status{
		Name:   d.Name,
		Status: scheduler.SchedulerStatus_Error,
		Nodes:  nodes,
	}
	summary := renderSummary(status, errors.New("test error"))
	require.Contains(t, summary, "test error")
	require.Contains(t, summary, d.Name)
}

func testRenderTable(t *testing.T, rp *Reporter, d *dag.DAG, nodes []*model.Node) {
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

func (m *mockMailer) SendMail(from string, to []string, subject, body string, _ []string) error {
	m.count += 1
	m.from = from
	m.to = to
	m.subject = subject
	m.body = body
	return nil
}
