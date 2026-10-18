// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/dagucloud/dagu/v2/api/v1"
	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	apiv1 "github.com/dagucloud/dagu/v2/internal/service/frontend/api/v1"
)

// hostPolicy rejects requests that address the server by a public domain it
// was not configured with. A DNS-rebinding page points its own domain at the
// server, so the browser sends that domain as both Host and Origin and the
// cross-origin check sees a same-origin request. Browsers keep credentials per
// site, so such a page gains nothing where authentication is required; the
// check applies to every route with auth.mode none, and otherwise only to the
// endpoint that creates the first admin.
type hostPolicy struct {
	allowedHosts []string
	// runtimeHosts returns names known only while the server runs, such as a
	// tunnel's.
	runtimeHosts func() []string
	guardAll     bool
	setupPath    string
}

func newHostPolicy(cfg *config.Config, setupPath, machineHostname string, runtimeHosts func() []string) hostPolicy {
	var allowed []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if domain, ok := strings.CutPrefix(name, "."); ok {
			if domain = hostName(domain); domain != "" {
				allowed = append(allowed, "."+domain)
			}
			return
		}
		if name = hostName(name); name != "" {
			allowed = append(allowed, name)
		}
	}
	add(cfg.Server.Host)
	if publicURL, err := url.Parse(cfg.Server.PublicURL); err == nil {
		add(publicURL.Hostname())
	}
	add(machineHostname)
	for _, entry := range cfg.Server.AllowedHosts {
		add(entry)
	}
	return hostPolicy{
		allowedHosts: allowed,
		runtimeHosts: runtimeHosts,
		guardAll:     cfg.Server.Auth.Mode == config.AuthModeNone,
		setupPath:    strings.TrimRight(setupPath, "/"),
	}
}

func (p hostPolicy) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !p.guards(r) {
			next.ServeHTTP(w, r)
			return
		}
		host := hostName(r.Host)
		if !p.allows(host) {
			apiv1.WriteErrorResponse(w, &apiv1.Error{
				HTTPStatus: http.StatusBadRequest,
				Code:       api.ErrorCodeBadRequest,
				Message:    fmt.Sprintf("host %q is not allowed; add it to allowed_hosts or set public_url", host),
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (p hostPolicy) guards(r *http.Request) bool {
	return p.guardAll || strings.TrimRight(r.URL.Path, "/") == p.setupPath
}

func (p hostPolicy) allows(host string) bool {
	if host == "" {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil || isPrivateName(host) {
		return true
	}
	if hostListMatches(p.allowedHosts, host) {
		return true
	}
	if p.runtimeHosts == nil {
		return false
	}
	var runtime []string
	for _, name := range p.runtimeHosts() {
		if name = hostName(name); name != "" {
			runtime = append(runtime, name)
		}
	}
	return hostListMatches(runtime, host)
}

// ianaTLDs is IANA's list of delegated top-level domains, from
// https://data.iana.org/TLD/tlds-alpha-by-domain.txt.
//
//go:embed iana_tlds.txt
var ianaTLDs string

// delegatedTLDs holds the lower-case top-level domains in ianaTLDs.
var delegatedTLDs = parseTLDList(ianaTLDs)

func parseTLDList(list string) map[string]struct{} {
	tlds := make(map[string]struct{})
	for line := range strings.Lines(list) {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			tlds[strings.ToLower(line)] = struct{}{}
		}
	}
	return tlds
}

// isPrivateName reports whether only local DNS answers for host: a single
// label, which resolvers qualify with local search domains and public DNS
// never publishes, or a name under a top-level domain ICANN has not
// delegated, such as localhost, local, lan, or internal. A remote page can
// rebind any other name, including the private suffixes of dynamic DNS
// services.
func isPrivateName(host string) bool {
	dot := strings.LastIndexByte(host, '.')
	if dot < 0 {
		return true
	}
	tld := host[dot+1:]
	// Either list may know of a delegation the other lacks. A name under the
	// top-level domain also matches wildcard-only rules such as *.ck, which
	// the top-level domain alone does not.
	if _, ok := delegatedTLDs[tld]; ok {
		return false
	}
	_, icann := publicsuffix.PublicSuffix("dagu." + tld)
	return !icann
}

// hostListMatches matches exact names, ".example.com" for a domain and its
// subdomains, and "*" for any name.
func hostListMatches(entries []string, host string) bool {
	for _, entry := range entries {
		if entry == "*" || entry == host {
			return true
		}
		if domain, ok := strings.CutPrefix(entry, "."); ok &&
			(host == domain || strings.HasSuffix(host, entry)) {
			return true
		}
	}
	return false
}

// hostName returns the name in a Host value or config entry in lower-case
// ASCII form, as browsers send it, without a port, IPv6 brackets, or a
// trailing dot.
func hostName(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.ToLower(strings.TrimSuffix(strings.Trim(value, "[]"), "."))
	if ascii, err := idna.Lookup.ToASCII(value); err == nil {
		return ascii
	}
	return value
}
