package mailer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHTMLContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "HTML document with DOCTYPE",
			content:  "<!DOCTYPE html><html><body><h1>Test</h1></body></html>",
			expected: true,
		},
		{
			name:     "HTML document without DOCTYPE",
			content:  "<html><body><h1>Test</h1></body></html>",
			expected: false,
		},
		{
			name:     "Plain text with newlines",
			content:  "This is plain text\nwith some\nline breaks",
			expected: false,
		},
		{
			name:     "Plain text single line",
			content:  "This is just plain text",
			expected: false,
		},
		{
			name:     "HTML with whitespace",
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
			name:     "Unix newlines",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "Windows newlines",
			input:    "Line 1\r\nLine 2\r\nLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "Mac newlines",
			input:    "Line 1\rLine 2\rLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "Mixed newlines",
			input:    "Line 1\nLine 2\r\nLine 3\rLine 4",
			expected: "Line 1<br />Line 2<br />Line 3<br />Line 4",
		},
		{
			name:     "Escaped newlines",
			input:    "Line 1\\nLine 2\\r\\nLine 3",
			expected: "Line 1<br />Line 2<br />Line 3",
		},
		{
			name:     "No newlines",
			input:    "Single line text",
			expected: "Single line text",
		},
		{
			name:     "Empty string",
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
			name: "Plain text email with newlines",
			emailBody: `Hello,

This is a plain text email.
It has multiple lines.

Best regards,
Dagu Team`,
			expectNewlineProcessed: true,
			description:            "Plain text should have newlines converted to <br /> tags",
		},
		{
			name: "HTML email with table",
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
			name: "Error message with angle brackets",
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
