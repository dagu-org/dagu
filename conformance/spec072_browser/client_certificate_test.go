// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec072_browser_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"software.sslmate.com/src/go-pkcs12"
)

// mtlsHost names the site that requires a client certificate. Chrome never
// resolves it: requests reach it through the upstream proxy, which maps it
// to the local server. Loopback addresses would bypass every proxy.
const mtlsHost = "mtls.test"

const certificatePassphrase = "conformance-passphrase"

// testCA issues certificates for one test.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T, name string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &testCA{cert: cert, key: key}
}

func (ca *testCA) issue(t *testing.T, commonName string, usage x509.ExtKeyUsage, dnsNames ...string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func (ca *testCA) pem() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw}))
}

// startMTLSSite serves a page greeting the client certificate's common name,
// and refuses connections without a certificate clientCA issued.
func startMTLSSite(t *testing.T, serverCA, clientCA *testCA) string {
	t.Helper()
	cert, key := serverCA.issue(t, mtlsHost, x509.ExtKeyUsageServerAuth, mtlsHost)
	pool := x509.NewCertPool()
	pool.AddCert(clientCA.cert)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<!doctype html><title>Portal</title><h1>Hello "+r.TLS.PeerCertificates[0].Subject.CommonName+"</h1>")
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{cert.Raw}, PrivateKey: key, Leaf: cert}},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server.Listener.Addr().String()
}

// upstreamProxy is a CONNECT proxy that sends mtlsHost to the local site and
// records the targets it was asked for.
type upstreamProxy struct {
	mu      sync.Mutex
	targets []string
}

func startUpstreamProxy(t *testing.T, siteAddr string) (*upstreamProxy, string) {
	t.Helper()
	proxy := &upstreamProxy{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		proxy.mu.Lock()
		proxy.targets = append(proxy.targets, r.Host)
		proxy.mu.Unlock()
		if r.Host != mtlsHost+":443" {
			http.Error(w, "unknown host", http.StatusBadGateway)
			return
		}
		upstream, err := net.Dial("tcp", siteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		client, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go func() { _, _ = io.Copy(upstream, client); _ = upstream.Close() }()
		_, _ = io.Copy(client, upstream)
		_ = client.Close()
	}))
	t.Cleanup(server.Close)
	return proxy, server.URL
}

func (p *upstreamProxy) connects() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.targets...)
}

// clientCertificateEnv sets up a site that requires a certificate clientCA
// issues, and writes the step's PKCS#12 file, issued by issuer, and the
// site's CA into the project.
func clientCertificateEnv(t *testing.T, b *browserEnv, clientCA, issuer *testCA) *upstreamProxy {
	t.Helper()
	serverCA := newTestCA(t, "Conformance Server CA")
	proxy, proxyURL := startUpstreamProxy(t, startMTLSSite(t, serverCA, clientCA))
	cert, key := issuer.issue(t, "client-1", x509.ExtKeyUsageClientAuth)
	data, err := pkcs12.Modern.Encode(key, cert, []*x509.Certificate{issuer.cert}, certificatePassphrase)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(b.dagu.ProjectPath("client.p12"), data, 0o600))
	b.dagu.WriteFile("ca.pem", serverCA.pem())
	b.env = append(b.env, "UPSTREAM_PROXY="+proxyURL, "CERT_PASSPHRASE="+certificatePassphrase)
	return proxy
}

// A site that requires a client certificate opens with the one the step
// names, reached through the proxy the step sets.
func TestBrowserClientCertificate(t *testing.T) {
	t.Parallel()

	b := newBrowserEnv(t)
	clientCA := newTestCA(t, "Conformance Client CA")
	proxy := clientCertificateEnv(t, b, clientCA, clientCA)
	result := b.dagu.RunWithEnv(b.env, "start", "client_certificate.yaml")
	result.ExpectExitCode(0)
	require.Contains(t, proxy.connects(), mtlsHost+":443")
	require.Zero(t, b.model.total())
}

// A certificate the site does not accept fails the step with the site's
// refusal.
func TestBrowserClientCertificateRejected(t *testing.T) {
	t.Parallel()

	b := newBrowserEnv(t)
	clientCertificateEnv(t, b, newTestCA(t, "Conformance Client CA"), newTestCA(t, "Other CA"))
	result := b.dagu.RunWithEnv(b.env, "start", "client_certificate.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains(mtlsHost + " rejected the client certificate")
}

// A wrong passphrase fails the step before a browser starts, without
// revealing the passphrase given.
func TestBrowserClientCertificateWrongPassphrase(t *testing.T) {
	t.Parallel()

	b := newBrowserEnv(t)
	clientCA := newTestCA(t, "Conformance Client CA")
	proxy := clientCertificateEnv(t, b, clientCA, clientCA)
	b.env = append(b.env, "CERT_PASSPHRASE=not-the-passphrase")
	result := b.dagu.RunWithEnv(b.env, "start", "client_certificate.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("with.browser.client_certificates[0]")
	result.ExpectStderrNotContains("not-the-passphrase")
	require.Empty(t, proxy.connects())
}
