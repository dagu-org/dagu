// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package certproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newForwardProxy is an HTTP proxy that sends every plain request to the
// server at target and records the URLs requested.
func newForwardProxy(t *testing.T, target string) *tunnelProxy {
	t.Helper()
	p := &tunnelProxy{}
	forward := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: target})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.requested = append(p.requested, r.URL.String())
		p.mu.Unlock()
		forward.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	p.addr = server.Listener.Addr().String()
	return p
}

// While the browser sends every request to the proxy, a plain HTTP request
// follows the device's proxy settings, except to a loopback destination,
// as it would from the browser.
func TestForwardHTTP(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t, "corp CA")
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "plain %s", r.URL.RequestURI())
	}))
	t.Cleanup(site.Close)

	tests := []struct {
		name string
		// routes is the PAC result, with %s for the forward proxy.
		routes  string
		url     string
		body    string
		proxied bool
	}{
		{name: "ThroughProxy", routes: "PROXY %s", url: "http://" + siteHost + "/page", body: "plain /page", proxied: true},
		{name: "Fallback", routes: "PROXY 127.0.0.1:1; PROXY %s", url: "http://" + siteHost + "/page", body: "plain /page", proxied: true},
		{name: "Loopback", routes: "PROXY %s", url: site.URL + "/page?a=1;b=2", body: "plain /page?a=1;b=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			upstream := newForwardProxy(t, siteAddr(site))
			pac := fmt.Sprintf(`function FindProxyForURL() { return "`+tt.routes+`"; }`, upstream.addr)
			ready := serveProxy(t, testConfig(t, pac, ca.clientEntry(t, testLabel, "127.0.0.1")))

			response, err := plainClient(ready).Get(tt.url)
			require.NoError(t, err)
			defer func() { _ = response.Body.Close() }()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, response.StatusCode)
			assert.Equal(t, tt.body, string(body))
			if tt.proxied {
				assert.Equal(t, []string{tt.url}, upstream.requests())
			} else {
				assert.Empty(t, upstream.requests())
			}
		})
	}

	t.Run("ProxyAuth", func(t *testing.T) {
		t.Parallel()
		authProxy := newTunnelProxy(t, siteAddr(site), http.StatusProxyAuthRequired)
		pac := `function FindProxyForURL() { return "PROXY ` + authProxy.addr + `"; }`
		cfg := testConfig(t, pac, ca.clientEntry(t, testLabel, "127.0.0.1"))
		ready := serveProxy(t, cfg)

		response, err := plainClient(ready).Get("http://" + siteHost + "/")
		require.NoError(t, err)
		_ = response.Body.Close()
		assert.Equal(t, http.StatusBadGateway, response.StatusCode)
		event := waitForEvent(t, ready, cfg.Token, EventProxyAuth)
		assert.Contains(t, event.Message, "could not reach "+siteHost)
	})
}

// A tunnel that does not carry TLS, as a ws: URL opens, is relayed as it
// is, for a host an entry names as for one the guard carries.
func TestCarryAllRelaysPlainTunnel(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t, "corp CA")
	upstream := newTunnelProxy(t, echoServer(t), http.StatusOK)
	pac := `function FindProxyForURL() { return "PROXY ` + upstream.addr + `"; }`
	guarded := guardConfig(t, ca, pac)
	guarded.TrustedCAs = []TrustedCA{trustedCA(ca, "127.0.0.1")}

	for name, cfg := range map[string]Config{
		"Named":   testConfig(t, pac, ca.clientEntry(t, testLabel, siteHost, "127.0.0.1")),
		"Guarded": guarded,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ready := serveProxy(t, cfg)
			require.True(t, ready.Manual)
			conn, status, err := connectThrough(fmt.Sprintf("127.0.0.1:%d", ready.Port), siteHost+":80")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)
			assert.Equal(t, "hello", readGreeting(t, conn))
		})
	}
}

// plainClient sends plain HTTP requests through the proxy, as the browser
// does with a manual proxy.
func plainClient(ready Ready) *http.Client {
	proxyURL := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", ready.Port)}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
}
