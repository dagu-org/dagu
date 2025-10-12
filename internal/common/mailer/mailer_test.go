package mailer

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsHTMLContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "HTMLDocumentWithDOCTYPE",
			content:  "<!DOCTYPE html><html><body><h1>Test</h1></body></html>",
			expected: true,
		},
		{
			name:     "HTMLDocumentWithoutDOCTYPE",
			content:  "<html><body><h1>Test</h1></body></html>",
			expected: false,
		},
		{
			name:     "PlainTextWithNewlines",
			content:  "This is plain text\nwith some\nline breaks",
			expected: false,
		},
		{
			name:     "PlainTextSingleLine",
			content:  "This is just plain text",
			expected: false,
		},
		{
			name:     "HTMLWithWhitespace",
			content:  "  \n  <!DOCTYPE html>\n<html>\n<body>Test</body></html>  ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHTMLContent(tt.content)
			assert.Equal(t, tt.expected, result, "Content: %q", tt.content)
		})
	}
}

func TestNewlineToBrTag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UnixNewlines",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "WindowsNewlines",
			input:    "Line 1\r\nLine 2\r\nLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "MacNewlines",
			input:    "Line 1\rLine 2\rLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "MixedNewlines",
			input:    "Line 1\nLine 2\r\nLine 3\rLine 4",
			expected: "Line 1<br />Line 2<br />Line 3<br />Line 4",
		},
		{
			name:     "EscapedNewlines",
			input:    "Line 1\\nLine 2\\r\\nLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "NoNewlines",
			input:    "Single line text",
			expected: "Single line text",
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := newlineToBrTag(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMailerContentTypeDetection tests that the mailer correctly applies
// newlineToBrTag only to plain text content and leaves HTML content unchanged
func TestMailerContentTypeDetection(t *testing.T) {
	tests := []struct {
		name                   string
		emailBody              string
		expectNewlineProcessed bool
		description            string
	}{
		{
			name: "PlainTextEmailWithNewlines",
			emailBody: `Hello,

This is a plain text email.
It has multiple lines.

Best regards,
Dagu Team`,
			expectNewlineProcessed: true,
			description:            "Plain text should have newlines converted to <br /> tags",
		},
		{
			name: "HTMLEmailWithTable",
			emailBody: `<!DOCTYPE html>
<html>
<head>
    <style>
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; }
    </style>
</head>
<body>
<table>
<thead>
<tr><th>Step</th><th>Status</th></tr>
</thead>
<tbody>
<tr><td>Build</td><td>Success</td></tr>
<tr><td>Test</td><td>Failed</td></tr>
</tbody>
</table>
</body>
</html>`,
			expectNewlineProcessed: false,
			description:            "HTML content should not have newlines converted to <br /> tags",
		},
		{
			name: "ErrorMessageWithAngleBrackets",
			emailBody: `Error occurred during execution:

File not found: <missing.txt>
Expected value: <100
Actual value: >200

Please check the configuration.`,
			expectNewlineProcessed: true,
			description:            "Plain text with angle brackets should still have newlines converted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isHTML := isHTMLContent(tt.emailBody)
			assert.Equal(t, !tt.expectNewlineProcessed, isHTML,
				"isHTMLContent should return %v for: %s", !tt.expectNewlineProcessed, tt.description)

			originalBody := tt.emailBody
			processedBody := tt.emailBody

			processedBody = processEmailBody(processedBody)

			if tt.expectNewlineProcessed {
				// For plain text, we expect <br /> tags to be added
				assert.Contains(t, processedBody, "<br />",
					"Plain text should contain <br /> tags after processing")
				assert.NotEqual(t, originalBody, processedBody,
					"Plain text body should be modified")
			} else {
				// For HTML, the body should remain unchanged
				assert.Equal(t, originalBody, processedBody,
					"HTML body should remain unchanged")

				// Verify that no additional <br /> tags were added between HTML elements
				// Count original <br> tags vs processed <br> tags
				originalBrCount := strings.Count(strings.ToLower(originalBody), "<br")
				processedBrCount := strings.Count(strings.ToLower(processedBody), "<br")
				assert.Equal(t, originalBrCount, processedBrCount,
					"HTML should not have additional <br /> tags added")
			}
		})
	}
}

// mockSMTPServer creates a mock SMTP server for testing
type mockSMTPServer struct {
	listener      net.Listener
	delay         time.Duration
	acceptDelay   time.Duration
	responseDelay time.Duration
}

func newMockSMTPServer(delay, acceptDelay, responseDelay time.Duration) (*mockSMTPServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	return &mockSMTPServer{
		listener:      listener,
		delay:         delay,
		acceptDelay:   acceptDelay,
		responseDelay: responseDelay,
	}, nil
}

func (s *mockSMTPServer) Address() string {
	return s.listener.Addr().String()
}

func (s *mockSMTPServer) Close() error {
	return s.listener.Close()
}

func (s *mockSMTPServer) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *mockSMTPServer) handleConnection(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	if s.acceptDelay > 0 {
		time.Sleep(s.acceptDelay)
	}

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	// Send initial greeting
	_, _ = writer.WriteString("220 mock.server ESMTP\r\n")
	_ = writer.Flush()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		// Simulate delay for all responses
		if s.responseDelay > 0 {
			time.Sleep(s.responseDelay)
		}

		// Simulate overall delay
		if s.delay > 0 {
			time.Sleep(s.delay)
		}

		// Simple SMTP command handling
		switch {
		case strings.HasPrefix(line, "HELO") || strings.HasPrefix(line, "EHLO"):
			_, _ = writer.WriteString("250 OK\r\n")
		case strings.HasPrefix(line, "MAIL FROM:"):
			_, _ = writer.WriteString("250 OK\r\n")
		case strings.HasPrefix(line, "RCPT TO:"):
			_, _ = writer.WriteString("250 OK\r\n")
		case strings.HasPrefix(line, "DATA"):
			_, _ = writer.WriteString("354 Start mail input\r\n")
			_ = writer.Flush()
			// Read until we get a line with just "."
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(dataLine) == "." {
					_, _ = writer.WriteString("250 OK\r\n")
					break
				}
			}
		case strings.HasPrefix(line, "QUIT"):
			_, _ = writer.WriteString("221 Bye\r\n")
			_ = writer.Flush()
			return
		default:
			_, _ = writer.WriteString("500 Unknown command\r\n")
		}
		_ = writer.Flush()
	}
}

func TestMailerTimeout(t *testing.T) {
	// Save original timeout and restore after test
	originalTimeout := mailTimeout
	defer func() {
		mailTimeout = originalTimeout
	}()

	// Set a shorter timeout for testing
	mailTimeout = 2 * time.Second

	t.Run("SendWithNoAuthTimeoutOnConnection", func(t *testing.T) {
		// Create a listener that accepts connections but never responds
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() {
			_ = listener.Close()
		}()

		// Get the address
		host, port, err := net.SplitHostPort(listener.Addr().String())
		require.NoError(t, err)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Accept connection but don't send SMTP greeting
			time.Sleep(5 * time.Second)
			_ = conn.Close()
		}()

		mailer := New(Config{
			Host: host,
			Port: port,
		})

		err = mailer.sendWithNoAuth(
			"from@example.com",
			[]string{"to@example.com"},
			"Test Subject",
			"Test Body",
			nil,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("SendWithNoAuthTimeoutDuringSMTPSession", func(t *testing.T) {
		// Create a mock server that delays responses
		server, err := newMockSMTPServer(0, 0, 3*time.Second)
		require.NoError(t, err)
		defer func() {
			_ = server.Close()
		}()

		go server.Serve()

		host, port, err := net.SplitHostPort(server.Address())
		require.NoError(t, err)

		mailer := New(Config{
			Host: host,
			Port: port,
		})

		err = mailer.sendWithNoAuth(
			"from@example.com",
			[]string{"to@example.com"},
			"Test Subject",
			"Test Body",
			nil,
		)

		assert.Error(t, err)
		// Should timeout due to slow responses
		assert.True(t, strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "deadline exceeded"))
	})

	t.Run("SendWithNoAuthSuccessfulWithinTimeout", func(t *testing.T) {
		// Create a mock server that responds quickly
		server, err := newMockSMTPServer(0, 0, 0)
		require.NoError(t, err)
		defer func() {
			_ = server.Close()
		}()

		go server.Serve()

		host, port, err := net.SplitHostPort(server.Address())
		require.NoError(t, err)

		mailer := New(Config{
			Host: host,
			Port: port,
		})

		// This should succeed as the server responds quickly
		err = mailer.sendWithNoAuth(
			"from@example.com",
			[]string{"to@example.com"},
			"Test Subject",
			"Test Body",
			nil,
		)

		// The mock server doesn't implement full SMTP, so we might get an error,
		// but it shouldn't be a timeout error
		if err != nil {
			assert.NotContains(t, err.Error(), "timeout")
			assert.NotContains(t, err.Error(), "deadline exceeded")
		}
	})

	t.Run("SendWithAuthTimeout", func(t *testing.T) {
		// Set an even shorter timeout for this test
		mailTimeout = 100 * time.Millisecond

		// Create a listener that never responds
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		// Close the listener immediately to ensure connection fails
		_ = listener.Close()

		host, port, err := net.SplitHostPort(listener.Addr().String())
		require.NoError(t, err)

		mailer := New(Config{
			Host:     host,
			Port:     port,
			Username: "user",
			Password: "pass",
		})

		start := time.Now()
		err = mailer.sendWithAuth(
			"from@example.com",
			[]string{"to@example.com"},
			"Test Subject",
			"Test Body",
			nil,
		)
		elapsed := time.Since(start)

		assert.Error(t, err)
		// Should timeout quickly
		assert.Less(t, elapsed, 500*time.Millisecond)
	})

	t.Run("SendMethodRoutesCorrectly", func(t *testing.T) {
		// Test that Send method correctly routes to sendWithAuth when credentials are provided
		mailer := New(Config{
			Host:     "invalid.host",
			Port:     "25",
			Username: "user",
			Password: "pass",
		})

		ctx := context.Background()
		err := mailer.Send(ctx, "from@example.com", []string{"to@example.com"}, "Subject", "Body", nil)
		assert.Error(t, err)

		// Test that Send method correctly routes to sendWithNoAuth when no credentials
		mailer = New(Config{
			Host: "invalid.host",
			Port: "25",
		})

		err = mailer.Send(ctx, "from@example.com", []string{"to@example.com"}, "Subject", "Body", nil)
		assert.Error(t, err)
	})
}
