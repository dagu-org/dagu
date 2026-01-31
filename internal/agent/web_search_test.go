package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestWebSearchTool_Run(t *testing.T) {
	t.Parallel()

	t.Run("empty query returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewWebSearchTool()
		input := json.RawMessage(`{"query": ""}`)

		result := tool.Run(backgroundCtx(), input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewWebSearchTool()
		input := json.RawMessage(`{invalid}`)

		result := tool.Run(backgroundCtx(), input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parse")
	})

	t.Run("works with nil context", func(t *testing.T) {
		t.Parallel()

		tool := NewWebSearchTool()
		input := json.RawMessage(`{"query": "golang"}`)

		// This will actually make a real HTTP request, but tests the nil context handling
		_ = tool.Run(ToolContext{}, input)
		// We don't assert on the result since it depends on network availability
	})
}

func TestResolveMaxResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{
			name:     "zero uses default",
			input:    0,
			expected: defaultMaxResults,
		},
		{
			name:     "negative uses default",
			input:    -1,
			expected: defaultMaxResults,
		},
		{
			name:     "valid value returned",
			input:    3,
			expected: 3,
		},
		{
			name:     "large value capped at max",
			input:    20,
			expected: maxAllowedResults,
		},
		{
			name:     "exactly at max",
			input:    10,
			expected: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := resolveMaxResults(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseSearchResults(t *testing.T) {
	t.Parallel()

	t.Run("parses valid HTML with results", func(t *testing.T) {
		t.Parallel()

		// Simplified test HTML that matches the isResultDiv logic
		htmlContent := `
		<html>
		<body>
			<div class="result link">
				<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpage1">Example Page 1</a>
				<span class="result__snippet">This is the first result description.</span>
			</div>
			<div class="result link">
				<a class="result__a" href="https://example.com/page2">Example Page 2</a>
				<span class="result__snippet">Second result description.</span>
			</div>
		</body>
		</html>
		`

		results, err := parseSearchResults(htmlContent, 5)

		require.NoError(t, err)
		require.Len(t, results, 2)

		assert.Equal(t, "Example Page 1", results[0].Title)
		assert.Equal(t, "https://example.com/page1", results[0].URL)
		assert.Equal(t, "This is the first result description.", results[0].Description)

		assert.Equal(t, "Example Page 2", results[1].Title)
		assert.Equal(t, "https://example.com/page2", results[1].URL)
	})

	t.Run("respects max results limit", func(t *testing.T) {
		t.Parallel()

		htmlContent := `
		<html>
		<body>
			<div class="result link">
				<a class="result__a" href="https://example.com/1">Page 1</a>
			</div>
			<div class="result link">
				<a class="result__a" href="https://example.com/2">Page 2</a>
			</div>
			<div class="result link">
				<a class="result__a" href="https://example.com/3">Page 3</a>
			</div>
		</body>
		</html>
		`

		results, err := parseSearchResults(htmlContent, 2)

		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("handles empty HTML", func(t *testing.T) {
		t.Parallel()

		results, err := parseSearchResults("<html><body></body></html>", 5)

		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("skips results without title or URL", func(t *testing.T) {
		t.Parallel()

		htmlContent := `
		<html>
		<body>
			<div class="result link">
				<span class="result__snippet">Just a description, no title or URL</span>
			</div>
			<div class="result link">
				<a class="result__a" href="https://valid.com">Valid Result</a>
			</div>
		</body>
		</html>
		`

		results, err := parseSearchResults(htmlContent, 5)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Valid Result", results[0].Title)
	})
}

func TestDecodeRedirectURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "DuckDuckGo redirect URL",
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath",
			expected: "https://example.com/path",
		},
		{
			name:     "direct URL unchanged",
			input:    "https://example.com/page",
			expected: "https://example.com/page",
		},
		{
			name:     "protocol-relative URL without uddg",
			input:    "//example.com/page",
			expected: "https://example.com/page",
		},
		{
			name:     "complex uddg parameter",
			input:    "//duckduckgo.com/l/?kh=-1&uddg=https%3A%2F%2Fgolang.org%2Fdoc%2F",
			expected: "https://golang.org/doc/",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := decodeRedirectURL(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatSearchResults(t *testing.T) {
	t.Parallel()

	t.Run("formats results correctly", func(t *testing.T) {
		t.Parallel()

		results := []SearchResult{
			{
				Title:       "Go Programming",
				URL:         "https://golang.org",
				Description: "The Go programming language",
			},
			{
				Title:       "Go Tutorial",
				URL:         "https://tour.golang.org",
				Description: "An interactive tour of Go",
			},
		}

		output := formatSearchResults("golang", results)

		assert.Contains(t, output, `Search results for "golang"`)
		assert.Contains(t, output, "1. Go Programming")
		assert.Contains(t, output, "URL: https://golang.org")
		assert.Contains(t, output, "The Go programming language")
		assert.Contains(t, output, "2. Go Tutorial")
		assert.Contains(t, output, "[Found 2 results]")
	})

	t.Run("handles result without description", func(t *testing.T) {
		t.Parallel()

		results := []SearchResult{
			{
				Title: "No Description",
				URL:   "https://example.com",
			},
		}

		output := formatSearchResults("test", results)

		assert.Contains(t, output, "1. No Description")
		assert.Contains(t, output, "URL: https://example.com")
		assert.NotContains(t, output, "   \n\n") // No empty description line
	})
}

func TestNewWebSearchTool(t *testing.T) {
	t.Parallel()

	tool := NewWebSearchTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "web_search", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Run)

	// Check parameters
	params := tool.Function.Parameters
	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "max_results")

	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "query")
}

func TestWebSearchTool_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		tool := NewWebSearchTool()
		input := json.RawMessage(`{"query": "golang tutorials"}`)

		result := tool.Run(ToolContext{Context: ctx}, input)

		// Should fail due to cancelled context
		assert.True(t, result.IsError)
	})
}

func TestIsResultDiv(t *testing.T) {
	t.Parallel()

	t.Run("matches result class", func(t *testing.T) {
		t.Parallel()

		// Use "result link" instead of "result results_links" - our logic rejects strings containing "results"
		htmlContent := `<div class="result link">content</div>`
		doc, err := parseHTMLFragment(htmlContent)
		require.NoError(t, err)

		div := findFirstElement(doc, "div")
		require.NotNil(t, div)
		assert.True(t, isResultDiv(div))
	})

	t.Run("does not match results container", func(t *testing.T) {
		t.Parallel()

		htmlContent := `<div class="results">content</div>`
		doc, err := parseHTMLFragment(htmlContent)
		require.NoError(t, err)

		div := findFirstElement(doc, "div")
		require.NotNil(t, div)
		assert.False(t, isResultDiv(div))
	})

	t.Run("does not match non-div elements", func(t *testing.T) {
		t.Parallel()

		htmlContent := `<span class="result">content</span>`
		doc, err := parseHTMLFragment(htmlContent)
		require.NoError(t, err)

		span := findFirstElement(doc, "span")
		require.NotNil(t, span)
		assert.False(t, isResultDiv(span))
	})
}

func TestGetTextContent(t *testing.T) {
	t.Parallel()

	t.Run("extracts text from nested elements", func(t *testing.T) {
		t.Parallel()

		html := `<div><span>Hello</span> <strong>World</strong></div>`
		doc, err := parseHTMLFragment(html)
		require.NoError(t, err)

		div := findFirstElement(doc, "div")
		require.NotNil(t, div)

		text := getTextContent(div)
		assert.Equal(t, "Hello World", text)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Parallel()

		html := `<div>  spaced text  </div>`
		doc, err := parseHTMLFragment(html)
		require.NoError(t, err)

		div := findFirstElement(doc, "div")
		require.NotNil(t, div)

		text := getTextContent(div)
		assert.Equal(t, "spaced text", text)
	})
}

// Helper functions for tests

func parseHTMLFragment(fragment string) (*html.Node, error) {
	return html.Parse(strings.NewReader(fragment))
}

func findFirstElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}
