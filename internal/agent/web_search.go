package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"golang.org/x/net/html"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "web_search",
		Label:          "Web Search",
		Description:    "Search the internet",
		DefaultEnabled: true,
		Factory:        func(_ ToolConfig) *AgentTool { return NewWebSearchTool() },
	})
}

const (
	defaultWebSearchTimeout = 30 * time.Second
	defaultMaxResults       = 5
	maxAllowedResults       = 10
	duckDuckGoURL           = "https://lite.duckduckgo.com/lite/"
	maxRetries              = 3
	retryWaitTime           = time.Second
)

// WebSearchToolInput defines the input parameters for the web search tool.
type WebSearchToolInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title       string
	URL         string
	Description string
}

// HTTPDoer is an interface for HTTP clients to enable testing.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewWebSearchTool creates a new web search tool for internet search.
func NewWebSearchTool() *AgentTool {
	return newWebSearchTool(nil, "")
}

// NewWebSearchToolWithClient creates a web search tool with a custom HTTP client for testing.
func NewWebSearchToolWithClient(client HTTPDoer, baseURL string) *AgentTool {
	return newWebSearchTool(client, baseURL)
}

func newWebSearchTool(client HTTPDoer, baseURL string) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "web_search",
				Description: "Search the internet using DuckDuckGo. Use this to find current information, documentation, tutorials, or any web content.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "The search query",
						},
						"max_results": map[string]any{
							"type":        "integer",
							"description": "Maximum number of results to return (default: 5, max: 10)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		Run: func(toolCtx ToolContext, input json.RawMessage) ToolOut {
			return webSearchRun(toolCtx, input, client, baseURL)
		},
	}
}

func webSearchRun(toolCtx ToolContext, input json.RawMessage, client HTTPDoer, baseURL string) ToolOut {
	var args WebSearchToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	if args.Query == "" {
		return toolError("Query is required")
	}

	maxResults := resolveMaxResults(args.MaxResults)

	parentCtx := toolCtx.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, defaultWebSearchTimeout)
	defer cancel()

	results, err := performSearch(ctx, args.Query, maxResults, client, baseURL)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return toolError("Search timed out after %v", defaultWebSearchTimeout)
		}
		return toolError("Search failed: %v", err)
	}

	if len(results) == 0 {
		return ToolOut{Content: fmt.Sprintf("No results found for %q", args.Query)}
	}

	return ToolOut{Content: formatSearchResults(args.Query, results)}
}

func resolveMaxResults(maxResults int) int {
	if maxResults <= 0 {
		return defaultMaxResults
	}
	return min(maxResults, maxAllowedResults)
}

func performSearch(ctx context.Context, query string, maxResults int, httpClient HTTPDoer, baseURL string) ([]SearchResult, error) {
	searchURL := duckDuckGoURL
	if baseURL != "" {
		searchURL = baseURL
	}

	doer := httpClient
	if doer == nil {
		doer = &http.Client{Timeout: defaultWebSearchTimeout}
	}

	var lastErr error
	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryWaitTime):
			}
		}

		form := url.Values{"q": {query}}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DaguBot/1.0)")

		resp, err := doer.Do(req)
		if err != nil {
			lastErr = err
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < 300 {
			return parseSearchResults(string(body), maxResults)
		}

		lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			continue
		}
		return nil, lastErr
	}

	return nil, lastErr
}

// parseSearchResults parses DuckDuckGo Lite's table-based HTML response.
// The Lite endpoint returns results as <a class="result-link"> for titles/URLs
// and <td class="result-snippet"> for descriptions.
func parseSearchResults(htmlContent string, maxResults int) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult
	var currentResult *SearchResult

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(results) >= maxResults {
			return
		}

		if n.Type == html.ElementNode {
			if n.Data == "a" && hasClass(n, "result-link") {
				title := getTextContent(n)
				href := getAttr(n, "href")
				if title != "" && href != "" {
					// Flush any previous result before starting a new one.
					if currentResult != nil && currentResult.Title != "" && currentResult.URL != "" {
						results = append(results, *currentResult)
					}
					currentResult = &SearchResult{
						Title: title,
						URL:   href,
					}
				}
			} else if n.Data == "td" && hasClass(n, "result-snippet") && currentResult != nil {
				currentResult.Description = getTextContent(n)
				// Snippet completes the result — flush it.
				if currentResult.Title != "" && currentResult.URL != "" {
					results = append(results, *currentResult)
				}
				currentResult = nil
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Flush any trailing result without a snippet.
	if currentResult != nil && currentResult.Title != "" && currentResult.URL != "" && len(results) < maxResults {
		results = append(results, *currentResult)
	}

	return results, nil
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, class) {
			return true
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func getTextContent(n *html.Node) string {
	var text strings.Builder
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return strings.TrimSpace(text.String())
}

func formatSearchResults(query string, results []SearchResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Search results for %q:\n\n", query)

	for i, r := range results {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, r.Title)
		fmt.Fprintf(&sb, "   URL: %s\n", r.URL)
		if r.Description != "" {
			fmt.Fprintf(&sb, "   %s\n", r.Description)
		}
		sb.WriteString("\n")
	}

	fmt.Fprintf(&sb, "[Found %d results]", len(results))
	return sb.String()
}
