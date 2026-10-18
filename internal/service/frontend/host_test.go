// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSetupPath = "/api/v1/auth/setup"

// A DNS-rebinding page sends its own domain as both Host and Origin, so the
// request looks same-origin. Without authentication, only the host name tells
// it apart from the real UI.
func TestHostPolicyRejectsUnknownBrowserHostsWithoutAuth(t *testing.T) {
	t.Parallel()

	handler := newTestHostPolicy(t, config.AuthModeNone, nil, nil)

	resp := serveHost(handler, browserRequest(http.MethodPost, "http://rebind.example:8080/api/v1/dag-runs"))

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	var body struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Contains(t, body.Message, `"rebind.example"`)
	assert.Contains(t, body.Message, "allowed_hosts")
}

func TestHostPolicyAllowsLocalAndConfiguredHosts(t *testing.T) {
	t.Parallel()

	handler := newTestHostPolicy(t, config.AuthModeNone, func(cfg *config.Config) {
		cfg.Server.Host = "dagu.internal"
		cfg.Server.PublicURL = "https://dagu.example.com/ui"
		cfg.Server.AllowedHosts = []string{"Ops.Example.NET:8080", ".corp.example"}
	}, func() []string { return []string{"dagu.tail1234.ts.net"} })

	for _, host := range []string{
		"localhost:8080",
		"app.localhost",
		"127.0.0.1:8080",
		"192.0.2.10",
		"[::1]:8080",
		"dagu.internal:8080",
		"dagu.example.com",
		"ops.example.net",
		"corp.example",
		"build.corp.example.",
		"dagu.tail1234.ts.net",
	} {
		t.Run(host, func(t *testing.T) {
			resp := serveHost(handler, browserRequest(http.MethodGet, "http://"+host+"/api/v1/dags"))
			assert.Equal(t, http.StatusNoContent, resp.Code)
		})
	}

	resp := serveHost(handler, browserRequest(http.MethodGet, "http://notcorp.example/api/v1/dags"))
	assert.Equal(t, http.StatusBadRequest, resp.Code, "a suffix entry matches whole labels only")
}

func TestHostPolicyAllowsMachineHostname(t *testing.T) {
	t.Parallel()

	policy := newHostPolicy(&config.Config{Server: config.Server{Auth: config.Auth{Mode: config.AuthModeNone}}},
		testSetupPath, "build-box.example.lan", nil)
	handler := policy.middleware(noContentHandler())

	for _, host := range []string{"build-box.example.lan", "build-box", "build-box.local"} {
		resp := serveHost(handler, browserRequest(http.MethodGet, "http://"+host+":8080/"))
		assert.Equal(t, http.StatusNoContent, resp.Code, host)
	}
}

func TestHostPolicyWildcardAllowsEveryHost(t *testing.T) {
	t.Parallel()

	handler := newTestHostPolicy(t, config.AuthModeNone, func(cfg *config.Config) {
		cfg.Server.AllowedHosts = []string{"*"}
	}, nil)

	resp := serveHost(handler, browserRequest(http.MethodPost, "http://rebind.example/api/v1/dag-runs"))
	assert.Equal(t, http.StatusNoContent, resp.Code)
}

// Non-browser clients cannot be driven by a rebinding page, and containers and
// services often reach Dagu by a name only their network knows.
func TestHostPolicyAllowsNonBrowserClients(t *testing.T) {
	t.Parallel()

	handler := newTestHostPolicy(t, config.AuthModeNone, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "http://dagu-ui.dagu.svc:8080/api/v1/dag-runs", nil)
	resp := serveHost(handler, req)
	assert.Equal(t, http.StatusNoContent, resp.Code)
}

// With authentication, a rebinding page holds no credentials for the server,
// so only the endpoint that creates the first admin needs the host check.
func TestHostPolicyGuardsOnlySetupWithAuth(t *testing.T) {
	t.Parallel()

	handler := newTestHostPolicy(t, config.AuthModeBuiltin, nil, nil)

	resp := serveHost(handler, browserRequest(http.MethodGet, "http://rebind.example/api/v1/dags"))
	assert.Equal(t, http.StatusNoContent, resp.Code)

	resp = serveHost(handler, browserRequest(http.MethodPost, "http://rebind.example"+testSetupPath+"/"))
	assert.Equal(t, http.StatusBadRequest, resp.Code)

	resp = serveHost(handler, browserRequest(http.MethodPost, "http://localhost:8080"+testSetupPath))
	assert.Equal(t, http.StatusNoContent, resp.Code)
}

func newTestHostPolicy(t *testing.T, mode config.AuthMode, configure func(*config.Config), runtimeHosts func() []string) http.Handler {
	t.Helper()
	cfg := &config.Config{Server: config.Server{Auth: config.Auth{Mode: mode}}}
	if configure != nil {
		configure(cfg)
	}
	return newHostPolicy(cfg, testSetupPath, "", runtimeHosts).middleware(noContentHandler())
}

func noContentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

// browserRequest builds a same-origin request as a browser sends it, with
// Fetch Metadata and, for unsafe methods, an Origin matching the URL.
func browserRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if method != http.MethodGet {
		req.Header.Set("Origin", "http://"+req.Host)
	}
	return req
}

func serveHost(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
