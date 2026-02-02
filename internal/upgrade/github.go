package upgrade

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	githubAPIURL = "https://api.github.com/repos/dagu-org/dagu/releases"

	defaultTimeout       = 30 * time.Second
	defaultRetryCount    = 3
	defaultRetryWaitTime = 1 * time.Second
	defaultRetryMaxWait  = 5 * time.Second
)

// Release represents a GitHub release.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	HTMLURL    string  `json:"html_url"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a GitHub release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// GitHubClient is an HTTP client for the GitHub Releases API.
type GitHubClient struct {
	client *resty.Client
}

// NewGitHubClient creates a new GitHub API client with retry logic.
func NewGitHubClient() *GitHubClient {
	client := resty.New().
		SetTimeout(defaultTimeout).
		SetRetryCount(defaultRetryCount).
		SetRetryWaitTime(defaultRetryWaitTime).
		SetRetryMaxWaitTime(defaultRetryMaxWait).
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("User-Agent", "dagu-upgrade-client").
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true // Retry on network errors
			}
			// Retry on rate limit and server errors
			code := r.StatusCode()
			return code == 429 || (code >= 500 && code <= 504)
		})

	return &GitHubClient{client: client}
}

// GetLatestRelease fetches the latest release from GitHub.
// If includePreRelease is true, it may return a pre-release version.
func (c *GitHubClient) GetLatestRelease(ctx context.Context, includePreRelease bool) (*Release, error) {
	if !includePreRelease {
		// Use the /latest endpoint which excludes pre-releases and drafts
		var release Release
		resp, err := c.client.R().
			SetContext(ctx).
			SetResult(&release).
			Get(githubAPIURL + "/latest")

		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest release: %w", err)
		}

		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode(), resp.String())
		}

		return &release, nil
	}

	// For pre-releases, we need to list all releases and find the newest
	var releases []Release
	resp, err := c.client.R().
		SetContext(ctx).
		SetResult(&releases).
		SetQueryParam("per_page", "10").
		Get(githubAPIURL)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode(), resp.String())
	}

	// Find the first non-draft release (may be pre-release)
	for i := range releases {
		if !releases[i].Draft {
			return &releases[i], nil
		}
	}

	return nil, fmt.Errorf("no releases found")
}

// GetRelease fetches a specific release by tag.
func (c *GitHubClient) GetRelease(ctx context.Context, tag string) (*Release, error) {
	// Ensure tag has 'v' prefix
	tag = NormalizeVersionTag(tag)

	var release Release
	resp, err := c.client.R().
		SetContext(ctx).
		SetResult(&release).
		Get(fmt.Sprintf("%s/tags/%s", githubAPIURL, tag))

	if err != nil {
		return nil, fmt.Errorf("failed to fetch release %s: %w", tag, err)
	}

	if resp.StatusCode() == 404 {
		return nil, fmt.Errorf("release %s not found", tag)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode(), resp.String())
	}

	return &release, nil
}

// GetChecksums downloads and parses checksums.txt from a release.
// Returns a map of filename to SHA256 hash.
func (c *GitHubClient) GetChecksums(ctx context.Context, release *Release) (map[string]string, error) {
	// Find checksums.txt asset
	var checksumsURL string
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			checksumsURL = asset.BrowserDownloadURL
			break
		}
	}

	if checksumsURL == "" {
		return nil, fmt.Errorf("checksums.txt not found in release %s", release.TagName)
	}

	resp, err := c.client.R().
		SetContext(ctx).
		Get(checksumsURL)

	if err != nil {
		return nil, fmt.Errorf("failed to download checksums.txt: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to download checksums.txt: status %d", resp.StatusCode())
	}

	return parseChecksums(resp.String())
}

// parseChecksums parses the checksums.txt format.
// Format: <sha256_hash>  <filename> (two spaces between hash and filename)
func parseChecksums(content string) (map[string]string, error) {
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Split on two spaces (standard sha256sum format)
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			// Try single space as fallback
			parts = strings.Fields(line)
			if len(parts) != 2 {
				continue
			}
		}

		hash := strings.TrimSpace(parts[0])
		filename := strings.TrimSpace(parts[1])

		if hash != "" && filename != "" {
			checksums[filename] = hash
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error parsing checksums: %w", err)
	}

	if len(checksums) == 0 {
		return nil, fmt.Errorf("no valid checksums found")
	}

	return checksums, nil
}

// FindAsset finds the appropriate asset for the given platform in a release.
func FindAsset(release *Release, platform Platform, version string) (*Asset, error) {
	expectedName := platform.AssetName(version)

	for i := range release.Assets {
		if release.Assets[i].Name == expectedName {
			return &release.Assets[i], nil
		}
	}

	return nil, fmt.Errorf("asset %s not found in release %s", expectedName, release.TagName)
}
