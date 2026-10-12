// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/browserhost"
	"github.com/dagucloud/dagu/v2/internal/browserhost/certproxy"
	cmnconfig "github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/cmn/hostpattern"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runtime/builtin/internal/agentstep"
)

const (
	// kindClientCertificate labels what the client certificate proxy reports.
	kindClientCertificate = "client_certificate"
	stepCertificateLabel  = "with.browser.client_certificates[%d]"
	hostCertificateLabel  = "browser.client_certificates[%d] in the Dagu config"
)

// clientAuthError matches the network errors of a site that asks for a TLS
// client certificate the browser does not present, or refuses the one it
// presents.
var clientAuthError = regexp.MustCompile(`^ERR_(SSL_CLIENT_AUTH_[A-Z_]+|BAD_SSL_CLIENT_AUTH_CERT)$`)

// connectionError matches the network errors a TLS connection the proxy
// could not complete shows in the browser.
var connectionError = regexp.MustCompile(`^ERR_(SSL_[A-Z_]+|BAD_SSL_[A-Z_]+|CERT_[A-Z_]+|EMPTY_RESPONSE|CONNECTION_(CLOSED|RESET)|TUNNEL_CONNECTION_FAILED|PROXY_CONNECTION_FAILED)$`)

// urlHost returns the host of address, or "" when it names none.
func urlHost(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// loadClientCertificates loads the certificates a browser presents: the
// step's, then the host's, so the step's entry wins for a host both name.
// The step's relative files resolve against workDir. Warnings report
// certificates outside their validity period.
func loadClientCertificates(ctx context.Context, step []cmnconfig.BrowserClientCertificate, workDir string) ([]certproxy.Entry, []string, error) {
	var entries []certproxy.Entry
	var warnings []string
	now := time.Now()
	for _, group := range []struct {
		label        string
		certificates []cmnconfig.BrowserClientCertificate
		baseDir      string
	}{
		{stepCertificateLabel, step, workDir},
		{hostCertificateLabel, cmnconfig.GetConfig(ctx).Browser.ClientCertificates, ""},
	} {
		for i, certificate := range group.certificates {
			entry, entryWarnings, err := certproxy.Load(fmt.Sprintf(group.label, i), certificate, group.baseDir, now)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, entry)
			warnings = append(warnings, entryWarnings...)
		}
	}
	return entries, warnings, nil
}

// withCertificatePassphrases returns secrets with the passphrases the step's
// certificates name, so they are masked and kept from the model like
// secrets even when written literally.
func withCertificatePassphrases(secrets map[string]string, certificates []cmnconfig.BrowserClientCertificate) map[string]string {
	for i, certificate := range certificates {
		if certificate.Passphrase == "" {
			continue
		}
		secrets = maps.Clone(secrets)
		if secrets == nil {
			secrets = map[string]string{}
		}
		secrets[fmt.Sprintf(stepCertificateLabel, i)+".passphrase"] = certificate.Passphrase
	}
	return secrets
}

// recordCertProxy stores in record where the certificate proxy runs, if the
// browser has one.
func recordCertProxy(record *browserhost.Record, proxy *certproxy.Handle) {
	if proxy == nil {
		return
	}
	record.CertProxyURL, record.CertProxyPID, record.CertProxyStartedAt = proxy.URL, proxy.PID, proxy.StartedAt
}

// recordedHandle locates the browser record describes.
func recordedHandle(record browserhost.Record) browserHandle {
	handle := browserHandle{
		CDPURL:           record.CDPURL,
		ExtensionID:      record.ExtensionID,
		ExtensionDir:     record.ExtensionDir,
		BrowserPID:       record.BrowserPID,
		BrowserStartedAt: record.BrowserStartedAt,
	}
	if record.CertProxyURL != "" {
		handle.CertProxy = &certproxy.Handle{URL: record.CertProxyURL, PID: record.CertProxyPID, StartedAt: record.CertProxyStartedAt}
	}
	return handle
}

// clientCertificateFailure explains the error page the browser shows, using
// what the certificate proxy reported for the page's host, or returns nil
// when the page is unrelated to client certificates. Without a known host,
// a proxy failure explains only an error that a failed TLS connection
// causes.
func clientCertificateFailure(page navigationFailure, failures []certproxy.Event, presenting bool) error {
	code := page.Code
	for _, failure := range slices.Backward(failures) {
		if page.Host != "" && hostpattern.Canonical(failure.Host) == hostpattern.Canonical(page.Host) ||
			page.Host == "" && connectionError.MatchString(code) {
			return fmt.Errorf("%s (the browser shows %s)", failure.Message, code)
		}
	}
	if !clientAuthError.MatchString(code) {
		return nil
	}
	if presenting {
		return fmt.Errorf("the site asks for a TLS client certificate (%s), but none was presented through Dagu; "+
			"check that its host matches the certificate's hosts, and that no Chrome enterprise policy sets the proxy", code)
	}
	return fmt.Errorf("the site asks for a TLS client certificate (%s); "+
		"set with.browser.client_certificates or browser.client_certificates in the Dagu config", code)
}

// certificateTrouble reports whether event is about a client certificate
// itself, rather than about reaching a host, so it belongs in a failed
// step's summary whichever host failed.
func certificateTrouble(event certproxy.Event) bool {
	switch event.Kind {
	case certproxy.EventClientCertificateRejected, certproxy.EventServerCertificate, certproxy.EventProxyAuth:
		return true
	}
	return false
}

// describeCertificateEvent writes what the certificate proxy reported in one
// line.
func describeCertificateEvent(event certproxy.Event) string {
	if event.Message != "" {
		return event.Message
	}
	return strings.TrimSpace(event.Kind + " " + event.Host)
}

// logCertificateEvent records what the certificate proxy reported while the
// operation at index ran.
func logCertificateEvent(t *agentstep.Timeline, index int, event certproxy.Event) {
	status := agentstep.StatusFailed
	if event.Kind == certproxy.EventConnect {
		status = agentstep.StatusCompleted
	}
	content := t.Masker.MaskString(describeCertificateEvent(event))
	_, _ = fmt.Fprintf(t.Log, "%s %s %s %s\n", t.Position(index), kindClientCertificate, status, content)
	t.AppendEvent(ir.AgentSessionEvent{Type: agentstep.EventOperation, Name: kindClientCertificate, Status: status, Content: content})
}
