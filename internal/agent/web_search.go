package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/html"
)

const (
	defaultWebSearchTimeout = 30 * time.Second
	defaultMaxResults       = 5
	maxAllowedResults       = 10
	duckDuckGoURL           = "https://html.duckduckgo.com/html/"
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

// NewWebSearchTool creates a new web search tool for internet search.
func NewWebSearchTool() *AgentTool {
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
		Run: webSearchRun,
	}
}

func webSearchRun(toolCtx ToolContext, input json.RawMessage) ToolOut {
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

	results, err := performSearch(ctx, args.Query, maxResults)
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

func performSearch(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	client := resty.New().
		SetTimeout(defaultWebSearchTimeout).
		SetRetryCount(maxRetries).
		SetRetryWaitTime(retryWaitTime).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			code := r.StatusCode()
			return code == 429 || code >= 500
		})

	resp, err := client.R().
		SetContext(ctx).
		SetFormData(map[string]string{"q": query}).
		SetHeader("User-Agent", "Mozilla/5.0 (compatible; DaguBot/1.0)").
		Post(duckDuckGoURL)

	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return parseSearchResults(resp.String(), maxResults)
}

func parseSearchResults(htmlContent string, maxResults int) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult
	var f func(*html.Node)
	f = func(n *html.Node) {
		if len(results) >= maxResults {
			return
		}

		if isResultDiv(n) {
			if result := extractResult(n); result != nil {
				results = append(results, *result)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return results, nil
}

func isResultDiv(n *html.Node) bool {
	if n.Type != html.ElementNode || n.Data != "div" {
		return false
	}
	for _, attr := range n.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, "result") && !strings.Contains(attr.Val, "results") {
			return true
		}
	}
	return false
}

func extractResult(n *html.Node) *SearchResult {
	result := &SearchResult{}

	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.Data == "a" && hasClass(node, "result__a") {
				result.Title = getTextContent(node)
				result.URL = extractURL(node)
			}
			if hasClass(node, "result__snippet") {
				result.Description = getTextContent(node)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)

	if result.Title == "" || result.URL == "" {
		return nil
	}

	return result
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, class) {
			return true
		}
	}
	return false
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

func extractURL(n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			return decodeRedirectURL(attr.Val)
		}
	}
	return ""
}

// decodeRedirectURL extracts the actual URL from DuckDuckGo's redirect URL.
// DuckDuckGo wraps URLs like: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com
func decodeRedirectURL(rawURL string) string {
	// Handle protocol-relative URLs
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Check for DuckDuckGo redirect
	if uddg := parsed.Query().Get("uddg"); uddg != "" {
		return uddg
	}

	return rawURL
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
