// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestReporter(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, rp *reporter, dAG *dag.DAG, nodes []*model.Node,
	){
		"create error mail":   testErrorMail,
		"no error mail":       testNoErrorMail,
		"create success mail": testSuccessMail,
		"create summary":      testRenderSummary,
		"create node list":    testRenderTable,
		"report summary":      testReportSummary,
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
				Steps: []dag.Step{
					{
						Name:    "test-step",
						Command: "true",
					},
				},
			}

			nodes := []*model.Node{
				{
					Step: dag.Step{
						Name:    "test-step",
						Command: "true",
						Args:    []string{"param-x"},
					},
					Status:     scheduler.NodeStatusRunning,
					StartedAt:  util.FormatTime(time.Now()),
					FinishedAt: util.FormatTime(time.Now().Add(time.Minute * 10)),
				},
			}

			rp := &reporter{
				sender: &mockSender{},
				logger: test.NewLogger(),
			}

			fn(t, rp, d, nodes)
		})
	}
}

func testErrorMail(t *testing.T, rp *reporter, dAG *dag.DAG, nodes []*model.Node) {
	dAG.MailOn.Failure = true
	dAG.MailOn.Success = false

	_ = rp.send(dAG, &model.Status{
		Status: scheduler.StatusError,
		Nodes:  nodes,
	}, fmt.Errorf("Error"))

	mock, ok := rp.sender.(*mockSender)
	require.True(t, ok)
	require.Contains(t, mock.subject, "Error")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testNoErrorMail(t *testing.T, rp *reporter, dAG *dag.DAG, nodes []*model.Node) {
	dAG.MailOn.Failure = false
	dAG.MailOn.Success = true

	err := rp.send(dAG, &model.Status{
		Status: scheduler.StatusError,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)

	mock, ok := rp.sender.(*mockSender)
	require.True(t, ok)
	require.Equal(t, 0, mock.count)
}

func testSuccessMail(t *testing.T, rp *reporter, dAG *dag.DAG, nodes []*model.Node) {
	dAG.MailOn.Failure = true
	dAG.MailOn.Success = true

	err := rp.send(dAG, &model.Status{
		Status: scheduler.StatusSuccess,
		Nodes:  nodes,
	}, nil)
	require.NoError(t, err)

	mock, ok := rp.sender.(*mockSender)
	require.True(t, ok)
	require.Contains(t, mock.subject, "Success")
	require.Contains(t, mock.subject, "test DAG")
	require.Equal(t, 1, mock.count)
}

func testReportSummary(t *testing.T, rp *reporter, _ *dag.DAG, nodes []*model.Node) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	rp.report(&model.Status{
		Status: scheduler.StatusSuccess,
		Nodes:  nodes,
	}, errors.New("test error"))

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, "test error")
}

func testRenderSummary(t *testing.T, _ *reporter, dAG *dag.DAG, nodes []*model.Node) {
	status := &model.Status{
		Name:   dAG.Name,
		Status: scheduler.StatusError,
		Nodes:  nodes,
	}
	summary := renderSummary(status, errors.New("test error"))
	require.Contains(t, summary, "test error")
	require.Contains(t, summary, dAG.Name)
}

func testRenderTable(t *testing.T, _ *reporter, _ *dag.DAG, nodes []*model.Node) {
	summary := renderTable(nodes)
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

func (m *mockSender) Send(from string, to []string, subject, body string, _ []string) error {
	m.count += 1
	m.from = from
	m.to = to
	m.subject = subject
	m.body = body
	return nil
}
