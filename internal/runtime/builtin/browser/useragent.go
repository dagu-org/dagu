// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	stagehand "github.com/browserbase/stagehand/packages/sdk-go/v4"
	"github.com/dagucloud/dagu/v2/internal/browserhost"
	"github.com/dagucloud/dagu/v2/internal/browserhost/certproxy"
	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/procutil"
)

// A browser without a window names itself HeadlessChrome in the user agent
// it sends, which sites read as automation: they refuse the page, or show
// another than the same browser with a window gets. A headless launch sends
// the user agent its executable sends with a window instead. That is
// learned from the executable, which may be any Chromium browser or a
// wrapper around one, and kept per executable, so it is learned again only
// once the executable is updated.

const (
	// userAgentsDirName holds, under the browser state, the user agent each
	// executable sends with a window.
	userAgentsDirName = "user-agents"
	userAgentsDirMode = 0o700
	userAgentFileMode = 0o600
	headlessProduct   = "HeadlessChrome/"
	headedProduct     = "Chrome/"
	// chromePathEnv names the executable the browser runtime starts when a
	// launch names none.
	chromePathEnv = "CHROME_PATH"
)

// errUserAgentUnread reports a browser that started without telling the
// user agent it sends.
var errUserAgentUnread = errors.New("the browser did not report its user agent")

// headedUserAgent is the user agent an executable sends with a window.
type headedUserAgent struct {
	// Executable is the executable as launches name it; empty for the one
	// the browser runtime finds.
	Executable string `json:"executable"`
	// Product is the browser and version the executable reported, which an
	// update changes.
	Product   string `json:"product"`
	UserAgent string `json:"userAgent"`
}

// userAgentKey names the executable a launch of executable starts, as kept
// user agents are keyed.
func userAgentKey(executable string) string {
	return cmp.Or(executable, os.Getenv(chromePathEnv))
}

// headedFrom returns the user agent the browser that reported version sends
// with a window, for the executable key names.
func headedFrom(key string, version browserhost.Version) headedUserAgent {
	return headedUserAgent{
		Executable: key,
		Product:    version.Product,
		UserAgent:  strings.Replace(version.UserAgent, headlessProduct, headedProduct, 1),
	}
}

// userAgentPath is the file that keeps the user agent of the executable key
// names.
func userAgentPath(browserDir, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(browserDir, userAgentsDirName, hex.EncodeToString(sum[:16])+".json")
}

// loadUserAgent returns the user agent kept for the executable key names.
// One that cannot be read counts as unknown.
func loadUserAgent(browserDir, key string) (headedUserAgent, bool) {
	if browserDir == "" {
		return headedUserAgent{}, false
	}
	data, err := fileutil.ReadFile(userAgentPath(browserDir, key))
	if err != nil {
		return headedUserAgent{}, false
	}
	var agent headedUserAgent
	if json.Unmarshal(data, &agent) != nil || agent.Executable != key || agent.Product == "" || agent.UserAgent == "" {
		return headedUserAgent{}, false
	}
	return agent, true
}

// keepUserAgent keeps agent for later launches of its executable. Launches
// that learn it together write the same file.
func keepUserAgent(browserDir string, agent headedUserAgent) error {
	if browserDir == "" {
		return nil
	}
	path := userAgentPath(browserDir, agent.Executable)
	if err := os.MkdirAll(filepath.Dir(path), userAgentsDirMode); err != nil {
		return err
	}
	return fileutil.WriteJSONAtomic(path, agent, userAgentFileMode)
}

// knownUserAgent returns the user agent kept for the executable of opts,
// which a headless launch sends. A headless launch learns it first when none
// is kept; an error is then the launch's, as when the executable does not
// start. The user agent is empty when it is not known.
func knownUserAgent(ctx context.Context, opts launchOptions, key string) (headedUserAgent, error) {
	if agent, ok := loadUserAgent(opts.BrowserDir, key); ok || !opts.Headless {
		return agent, nil
	}
	agent, err := learnUserAgent(ctx, opts, key)
	if errors.Is(err, errUserAgentUnread) {
		warnUserAgent(opts, err)
		return headedUserAgent{}, nil
	}
	if err != nil {
		return headedUserAgent{}, err
	}
	keepLearned(opts, agent)
	return agent, nil
}

// confirmUserAgent checks the browser started after sending sent against the
// executable it runs, and keeps what it learns for later launches. An
// executable updated since its user agent was learned is asked again, and
// the browser is started again only when the user agent changed with it. A
// browser whose version cannot be read is left as it is.
func (l stagehandLauncher) confirmUserAgent(ctx context.Context, opts launchOptions, key string, known headedUserAgent, sent string, started startedBrowser) (startedBrowser, error) {
	version, err := boundCall(ctx, unstartedCallTimeout, func(ctx context.Context) (browserhost.Version, error) {
		return browserhost.BrowserVersion(ctx, started.cdpURL)
	})
	if err != nil {
		warnUserAgent(opts, err)
		return started, nil
	}
	if known.Product == version.Product {
		return started, nil
	}
	learned := headedFrom(key, version)
	if sent != "" {
		// The browser sends the user agent it was given, so it cannot tell
		// its own.
		if learned, err = learnUserAgent(ctx, opts, key); err != nil {
			warnUserAgent(opts, err)
			return started, nil
		}
	}
	keepLearned(opts, learned)
	if !opts.Headless || learned.UserAgent == cmp.Or(sent, version.UserAgent) {
		return started, nil
	}
	if err := started.end(ctx); err != nil {
		return startedBrowser{}, fmt.Errorf("start the browser again with its user agent: %w", err)
	}
	return l.start(ctx, opts, learned.UserAgent)
}

// learnUserAgent starts the executable of opts without a window, on a
// profile of its own and without a proxy, to learn the user agent it sends
// with one. The error is the launch's when the browser does not start, and
// wraps errUserAgentUnread when it started without telling.
func learnUserAgent(ctx context.Context, opts launchOptions, key string) (headedUserAgent, error) {
	dir, err := os.MkdirTemp("", "dagu-browser-probe-")
	if err != nil {
		return headedUserAgent{}, fmt.Errorf("create browser profile directory: %w", err)
	}
	// A browser that just exited can still hold profile files open for a
	// moment on Windows.
	defer func() { _ = fileutil.RemoveAll(dir) }()
	port, err := freeLoopbackPort()
	if err != nil {
		return headedUserAgent{}, err
	}
	cdpURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	browser, err := stagehand.LaunchLocalBrowser(ctx, &stagehand.LocalBrowserLaunchOptions{
		ExecutablePath:  opts.Executable,
		Headless:        true,
		Port:            port,
		UserDataDir:     dir,
		ChromiumSandbox: new(!opts.NoSandbox),
	})
	if err != nil {
		launchErr := fmt.Errorf("launch browser: %w%s", err, sandboxHint(opts.NoSandbox))
		return headedUserAgent{}, errors.Join(launchErr, endUnstartedBrowser(ctx, nil, cdpURL, dir))
	}
	version, err := boundCall(ctx, unstartedCallTimeout, func(ctx context.Context) (browserhost.Version, error) {
		return browserhost.BrowserVersion(ctx, cdpURL)
	})
	if endErr := (startedBrowser{browser: browser, cdpURL: cdpURL}).end(ctx); endErr != nil && opts.Warn != nil {
		opts.Warn("end the browser that told its user agent: " + endErr.Error())
	}
	if err != nil {
		return headedUserAgent{}, fmt.Errorf("%w: %w", errUserAgentUnread, err)
	}
	return headedFrom(key, version), nil
}

// keepLearned keeps a user agent a launch learned, warning when it cannot.
func keepLearned(opts launchOptions, agent headedUserAgent) {
	if err := keepUserAgent(opts.BrowserDir, agent); err != nil && opts.Warn != nil {
		opts.Warn(fmt.Sprintf("keep the browser's user agent: %v; the next headless launch learns it again", err))
	}
}

// warnUserAgent reports a user agent a launch could not learn.
func warnUserAgent(opts launchOptions, err error) {
	if opts.Warn != nil && opts.Headless {
		opts.Warn(fmt.Sprintf("learn the user agent the browser sends with a window: %v; pages may see it as a browser without one", err))
	}
}

// startedBrowser is a browser a launch started, before its runtime starts.
type startedBrowser struct {
	browser *stagehand.Browser
	cdpURL  string
	// proxy presents the browser's client certificates; nil when it presents
	// none.
	proxy *certproxy.Handle
}

// end terminates the browser, waiting for its processes to exit so its
// profile is free, and then its certificate proxy.
func (b startedBrowser) end(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	pid, _ := boundCall(ctx, unstartedCallTimeout, func(ctx context.Context) (int, error) {
		return browserhost.BrowserProcessID(ctx, b.cdpURL)
	})
	startedAt, _ := procutil.StartTime(pid)
	return errors.Join(closeBrowser(ctx, pid, startedAt, b.browser.Close), killCertProxy(b.proxy))
}

// warnOnce returns warn reporting each message once, as a browser started
// again reports what its first start did.
func warnOnce(warn func(string)) func(string) {
	if warn == nil {
		return nil
	}
	seen := map[string]bool{}
	return func(message string) {
		if !seen[message] {
			seen[message] = true
			warn(message)
		}
	}
}
