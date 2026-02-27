package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// liteHTML returns HTML in DuckDuckGo Lite's table-based format.
func liteHTML(results ...struct{ title, href, snippet string }) string {
	var sb strings.Builder
	sb.WriteString(`<html><body><table>`)
	for i, r := range results {
		sb.WriteString(`<tr><td>`)
		sb.WriteString(strings.Repeat(" ", i)) // number column
		sb.WriteString(`</td><td><a rel="nofollow" href="`)
		sb.WriteString(r.href)
		sb.WriteString(`" class="result-link">`)
		sb.WriteString(r.title)
		sb.WriteString(`</a></td></tr>`)
		if r.snippet != "" {
			sb.WriteString(`<tr><td></td><td class="result-snippet">`)
			sb.WriteString(r.snippet)
			sb.WriteString(`</td></tr>`)
		}
	}
	sb.WriteString(`</table></body></html>`)
	return sb.String()
}

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

	t.Run("accepts 202 status code", func(t *testing.T) {
		t.Parallel()

		body := liteHTML(struct{ title, href, snippet string }{
			"Result", "https://example.com", "",
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted) // 202
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		tool := NewWebSearchToolWithClient(server.Client(), server.URL)
		result := tool.Run(backgroundCtx(), json.RawMessage(`{"query": "test"}`))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "example.com")
	})

	t.Run("works with nil context using mock server", func(t *testing.T) {
		t.Parallel()

		body := liteHTML(struct{ title, href, snippet string }{
			"Go", "https://golang.org", "",
		})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		toolWithClient := NewWebSearchToolWithClient(server.Client(), server.URL)
		result := toolWithClient.Run(ToolContext{}, json.RawMessage(`{"query": "golang"}`))
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "golang.org")
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

	t.Run("parses valid Lite HTML with results", func(t *testing.T) {
		t.Parallel()

		htmlContent := liteHTML(
			struct{ title, href, snippet string }{
				"Example Page 1", "https://example.com/page1", "This is the first result description.",
			},
			struct{ title, href, snippet string }{
				"Example Page 2", "https://example.com/page2", "Second result description.",
			},
		)

		results, err := parseSearchResults(htmlContent, 5)

		require.NoError(t, err)
		require.Len(t, results, 2)

		assert.Equal(t, "Example Page 1", results[0].Title)
		assert.Equal(t, "https://example.com/page1", results[0].URL)
		assert.Equal(t, "This is the first result description.", results[0].Description)

		assert.Equal(t, "Example Page 2", results[1].Title)
		assert.Equal(t, "https://example.com/page2", results[1].URL)
		assert.Equal(t, "Second result description.", results[1].Description)
	})

	t.Run("respects max results limit", func(t *testing.T) {
		t.Parallel()

		htmlContent := liteHTML(
			struct{ title, href, snippet string }{"Page 1", "https://example.com/1", "desc"},
			struct{ title, href, snippet string }{"Page 2", "https://example.com/2", "desc"},
			struct{ title, href, snippet string }{"Page 3", "https://example.com/3", "desc"},
		)

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

	t.Run("handles result without snippet", func(t *testing.T) {
		t.Parallel()

		htmlContent := liteHTML(
			struct{ title, href, snippet string }{"No Snippet", "https://example.com", ""},
		)

		results, err := parseSearchResults(htmlContent, 5)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "No Snippet", results[0].Title)
		assert.Equal(t, "https://example.com", results[0].URL)
		assert.Empty(t, results[0].Description)
	})

	t.Run("skips links without href", func(t *testing.T) {
		t.Parallel()

		htmlContent := `<html><body><table>
			<tr><td><a class="result-link">No href</a></td></tr>
			<tr><td><a class="result-link" href="https://valid.com">Valid</a></td></tr>
			<tr><td class="result-snippet">desc</td></tr>
		</table></body></html>`

		results, err := parseSearchResults(htmlContent, 5)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "Valid", results[0].Title)
	})
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
		assert.NotContains(t, output, "   \n\n")
	})
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
		assert.True(t, result.IsError)
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
