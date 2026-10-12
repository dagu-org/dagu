// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package browser

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/dagucloud/dagu/v2/internal/browserhost"
	"github.com/dagucloud/dagu/v2/internal/browserhost/certproxy"
	cmnconfig "github.com/dagucloud/dagu/v2/internal/cmn/config"
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

// clientCertificateFailure explains the error page the browser shows with
// code, using the failures the certificate proxy reported, or returns nil
// when code is unrelated to client certificates.
func clientCertificateFailure(code string, failures []certproxy.Event, presenting bool) error {
	if len(failures) > 0 {
		return fmt.Errorf("%s (the browser shows %s)", failures[len(failures)-1].Message, code)
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
