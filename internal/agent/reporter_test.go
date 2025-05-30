package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
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
					To:     "to@mailer.com",
				},
				InfoMail: &digraph.MailConfig{
					Prefix: "Success: ",
					From:   "from@mailer.com",
					To:     "to@mailer.com",
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
					Status:     scheduler.NodeStatusRunning,
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

func testErrorMail(t *testing.T, rp *reporter, mock *mockSender, dag *digraph.DAG, nodes []*models.Node) {
	dag.MailOn.Failure = true
	dag.MailOn.Success = false

	_ = rp.send(context.Background(), dag, models.DAGRunStatus{
		Status: scheduler.StatusError,
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
		Status: scheduler.StatusError,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, 0, mock.count)
}

func testSuccessMail(t *testing.T, rp *reporter, mock *mockSender, dag *digraph.DAG, nodes []*models.Node) {
	dag.MailOn.Failure = true
	dag.MailOn.Success = true

	err := rp.send(context.Background(), dag, models.DAGRunStatus{
		Status: scheduler.StatusSuccess,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)

	require.Contains(t, mock.subject, "Success")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testRenderSummary(t *testing.T, _ *reporter, _ *mockSender, dag *digraph.DAG, _ []*models.Node) {
	status := models.NewStatusBuilder(dag).Create("run-id", scheduler.StatusError, 0, time.Now())
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
