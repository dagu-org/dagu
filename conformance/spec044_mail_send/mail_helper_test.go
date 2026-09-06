// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec044_mail_send_test

import (
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// Receive one message locally; MIME encoding and authentication belong in mailer tests.
func receiveMail(t *testing.T) ([]string, <-chan string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	messages := make(chan string, 1)
	done := make(chan struct{})
	timeout := harness.WaitTimeout(t)
	t.Cleanup(func() { _ = listener.Close(); <-done })

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(timeout))
		wire := textproto.NewConn(conn)
		const (
			greeting     = "220 localhost"
			accepted     = "250 OK"
			readyForData = "354 Send message"
			goodbye      = "221 Bye"
		)
		if err := wire.PrintfLine(greeting); err != nil {
			return
		}
		for {
			line, err := wire.ReadLine()
			if err != nil {
				return
			}
			switch {
			case line == "DATA":
				if err := wire.PrintfLine(readyForData); err != nil {
					return
				}
				body, err := wire.ReadDotBytes()
				if err != nil {
					return
				}
				messages <- string(body)
			case line == "QUIT":
				_ = wire.PrintfLine(goodbye)
				return
			case strings.HasPrefix(line, "EHLO "), strings.HasPrefix(line, "HELO "),
				strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
			default:
				t.Errorf("unexpected SMTP command: %s", line)
				return
			}
			if err := wire.PrintfLine(accepted); err != nil {
				return
			}
		}
	}()
	return []string{"SMTP_HOST=" + host, "SMTP_PORT=" + port}, messages
}
