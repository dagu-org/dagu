// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package certproxy

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/hostpattern"
)

const (
	// handshakeTimeout bounds the browser's TLS handshake with the proxy,
	// which includes reaching the server and the handshake with it.
	handshakeTimeout = 30 * time.Second
	// readHeaderTimeout bounds reading a request to the proxy.
	readHeaderTimeout = 10 * time.Second
	// leafLifetime and leafBackdate set the validity of the certificates
	// the browser is shown, which it accepts by their key alone.
	leafLifetime = 365 * 24 * time.Hour
	leafBackdate = time.Hour
	// leafSerialBits sizes the random serial numbers of those
	// certificates.
	leafSerialBits = 128
	// maxCommonNameLength is the longest common name a certificate holds.
	maxCommonNameLength = 64
	// relayBufferBytes sizes each read relayed from a server.
	relayBufferBytes = 32 << 10
	// defaultTLSPort is the port of a CONNECT authority that names none.
	defaultTLSPort = "443"
	// remoteErrorOp marks the error of an alert a TLS peer sent.
	remoteErrorOp = "remote error"
	// alertPrefix starts the text of a TLS alert.
	alertPrefix        = "tls: "
	connectEstablished = "HTTP/1.1 200 Connection Established\r\n\r\n"
)

// Control paths, under the token.
const (
	eventsPath  = "events"
	pendingPath = "pending"
	pacPath     = "proxy.pac"
	afterParam  = "after"
)

// rejectionAlerts are the TLS alerts by which a server refuses a client
// certificate: handshake failure, bad, unsupported, revoked, expired, and
// unknown certificate, unknown certificate authority, access denied, and
// certificate required.
var rejectionAlerts = []tls.AlertError{40, 42, 43, 44, 45, 46, 48, 49, 116}

// server is a running proxy.
type server struct {
	// ctx ends when the proxy stops.
	ctx       context.Context
	token     string
	entries   []*upstreamEntry
	pac       *pacEvaluator
	issuer    *leafIssuer
	events    *eventLog
	pacScript string // served when too large for a data: URL
	pending   *pendingLog
	// guardEntry presents no certificate, to the hosts the guard carries.
	guardEntry *upstreamEntry

	mu      sync.Mutex
	closed  bool
	tunnels map[net.Conn]struct{}
	// guard accepts the browser's other https: connections; nil without a
	// guard, or once it stopped.
	guard net.Listener
}

// upstreamEntry is an Entry ready to present.
type upstreamEntry struct {
	label string
	hosts []string
	// cert is empty for the guard, which presents no certificate.
	cert     tls.Certificate
	roots    *x509.CertPool
	sessions tls.ClientSessionCache
}

// newGuardEntry returns the entry of the hosts the guard carries, which
// presents no certificate. It trusts the system's roots and the CA
// certificates the entries name, which a private CA's other sites need.
func newGuardEntry(entries []Entry) *upstreamEntry {
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	for _, entry := range entries {
		roots.AppendCertsFromPEM([]byte(entry.CAPEM))
	}
	return &upstreamEntry{roots: roots, sessions: tls.NewLRUClientSessionCache(0)}
}

// presents reports whether the entry has a certificate to present.
func (e *upstreamEntry) presents() bool {
	return len(e.cert.Certificate) > 0
}

func newUpstreamEntry(entry Entry) (*upstreamEntry, error) {
	cert, err := tls.X509KeyPair([]byte(entry.ChainPEM), []byte(entry.KeyPEM))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", entry.Label, err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if entry.CAPEM != "" && !roots.AppendCertsFromPEM([]byte(entry.CAPEM)) {
		return nil, fmt.Errorf("%s: ca_pem holds no certificate", entry.Label)
	}
	return &upstreamEntry{
		label:    entry.Label,
		hosts:    entry.Hosts,
		cert:     cert,
		roots:    roots,
		sessions: tls.NewLRUClientSessionCache(0),
	}, nil
}

// upstreamConn is a TLS connection to a server.
type upstreamConn struct {
	*tls.Conn
	via route
	// requested is set once the server asked for a client certificate,
	// which it can do again in a renegotiation after the handshake.
	requested *atomic.Bool
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r, false)
		return
	}
	// The control paths answer only requests made to the proxy itself, not
	// requests for another site that a client sends through it.
	if r.Method != http.MethodGet || r.URL.IsAbs() {
		http.NotFound(w, r)
		return
	}
	switch r.URL.Path {
	case s.controlPath(eventsPath):
		s.serveEvents(w, r)
	case s.controlPath(pendingPath):
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.pending.list())
	case s.controlPath(pacPath):
		if s.pacScript == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", pacContentType)
		_, _ = io.WriteString(w, s.pacScript)
	default:
		http.NotFound(w, r)
	}
}

func (s *server) controlPath(name string) string {
	return "/" + s.token + "/" + name
}

func (s *server) serveEvents(w http.ResponseWriter, r *http.Request) {
	var after int64
	if text := r.URL.Query().Get(afterParam); text != "" {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			http.Error(w, "after must be a number", http.StatusBadRequest)
			return
		}
		after = n
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.events.after(after))
}

// guardHandler serves the guard, which tunnels to any host.
type guardHandler struct{ s *server }

func (g guardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.NotFound(w, r)
		return
	}
	g.s.handleConnect(w, r, true)
}

// handleConnect opens a tunnel to the host a CONNECT request names, ending
// the browser's TLS in the proxy. The certificate port tunnels only to the
// hosts the entries name; the guard tunnels to any host, presenting no
// certificate to the others.
func (s *server) handleConnect(w http.ResponseWriter, r *http.Request, guard bool) {
	host, port := splitAuthority(r.Host)
	if host == "" {
		http.Error(w, "the CONNECT request names no host", http.StatusBadRequest)
		return
	}
	entry := s.entryFor(host)
	if entry == nil {
		if !guard {
			s.events.add(EventRefused, host, "", fmt.Sprintf("the certificate proxy refused a connection to %s, which no client certificate names", host))
			http.Error(w, "no client certificate names this host", http.StatusForbidden)
			return
		}
		entry = s.guardEntry
	}
	pending := s.pending.start(host)
	ctx, cancel := context.WithTimeout(s.ctx, handshakeTimeout)
	raw, via, err := s.reach(ctx, host, port, entry)
	cancel()
	if err != nil {
		s.pending.end(pending)
		if guard && errors.Is(err, errProxyAuth) {
			// A reset connection makes the browser try the route after the
			// guard; a closed guard keeps it on that route.
			s.stopGuard(host)
			resetConnection(w)
			return
		}
		kind := EventUpstream
		if errors.Is(err, errProxyAuth) {
			kind = EventProxyAuth
		}
		s.events.add(kind, host, entry.label, fmt.Sprintf("the certificate proxy could not reach %s: %s", host, oneLine(err)))
		http.Error(w, "the certificate proxy could not reach the site", http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		s.pending.end(pending)
		_ = raw.Close()
		http.Error(w, "the connection cannot be tunneled", http.StatusInternalServerError)
		return
	}
	conn, buffered, err := hijacker.Hijack()
	if err != nil {
		s.pending.end(pending)
		_ = raw.Close()
		return
	}
	if _, err := io.WriteString(conn, connectEstablished); err != nil {
		s.pending.end(pending)
		_ = raw.Close()
		_ = conn.Close()
		return
	}
	client := conn
	if buffered.Reader.Buffered() > 0 {
		client = &bufferedConn{Conn: conn, reader: buffered.Reader}
	}
	s.tunnel(client, raw, via, host, entry, pending)
}

// stopGuard closes the guard, so the browser sends the requests it carried
// on the route after it, and records why.
func (s *server) stopGuard(host string) {
	s.mu.Lock()
	guard := s.guard
	s.guard = nil
	s.mu.Unlock()
	if guard == nil {
		return
	}
	_ = guard.Close()
	s.events.add(EventGuardOff, host, "", fmt.Sprintf(
		"the device's proxy asks %s for credentials the certificate proxy cannot give, so sites without a client certificate "+
			"now go through the browser's own route; one of them that asks for a client certificate can make the system prompt for it", host))
}

// resetConnection resets the connection of the request w answers, which
// browsers take as a proxy that cannot be reached.
func resetConnection(w http.ResponseWriter) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = conn.Close()
}

// splitAuthority returns the host, canonical, and port a CONNECT authority
// names.
func splitAuthority(authority string) (string, string) {
	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		host, port = authority, defaultTLSPort
	}
	return hostpattern.Canonical(host), port
}

// entryFor returns the first entry whose hosts match host, or nil.
func (s *server) entryFor(host string) *upstreamEntry {
	for _, entry := range s.entries {
		if hostpattern.MatchAny(host, entry.hosts) {
			return entry
		}
	}
	return nil
}

// tunnel relays the browser's connection to the server reached over raw.
// The browser's handshake waits for the one with the server, so it learns
// the server's application protocol, and fails when that one does.
func (s *server) tunnel(client, raw net.Conn, via route, host string, entry *upstreamEntry, pending int64) {
	defer s.pending.end(pending)
	if !s.track(client) {
		_ = client.Close()
		_ = raw.Close()
		return
	}
	defer s.untrack(client)

	var upstream *upstreamConn
	downstream := tls.Server(client, &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			s.pending.set(pending, PhaseHandshake)
			conn, err := s.handshake(hello.Context(), raw, via, host, entry, hello.SupportedProtos)
			if err != nil {
				return nil, err
			}
			upstream = conn
			leaf, err := s.issuer.leaf(host)
			if err != nil {
				return nil, err
			}
			config := &tls.Config{Certificates: []tls.Certificate{*leaf}, MinVersion: tls.VersionTLS12}
			if protocol := conn.ConnectionState().NegotiatedProtocol; protocol != "" {
				config.NextProtos = []string{protocol}
			}
			return config, nil
		},
	})
	ctx, cancel := context.WithTimeout(s.ctx, handshakeTimeout)
	err := downstream.HandshakeContext(ctx)
	cancel()
	if err != nil {
		_ = client.Close()
		_ = raw.Close()
		return
	}
	s.pending.set(pending, PhaseAnswer)
	s.reportConnected(host, entry, upstream)
	s.relay(downstream, upstream, host, entry, pending)
}

// reach connects to host and port on the route the device's proxy settings
// give it.
func (s *server) reach(ctx context.Context, host, port string, entry *upstreamEntry) (net.Conn, route, error) {
	found := s.pac.routes(ctx, pacRequestURL(host, port), host)
	for _, skipped := range found.skipped {
		s.events.add(EventUpstream, host, entry.label, fmt.Sprintf(
			"the device's proxy settings route %s through %s, which the certificate proxy does not support; it tries the next route", host, skipped))
	}
	return dialRoutes(ctx, found.routes, net.JoinHostPort(host, port), s.proxyTLS)
}

// proxyTLS returns the TLS configuration for an HTTPS proxy at host. An
// entry whose hosts match the proxy's presents its certificate to it, and
// its ca_file is trusted for it.
func (s *server) proxyTLS(host string) *tls.Config {
	config := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if entry := s.entryFor(hostpattern.Canonical(host)); entry != nil {
		config.RootCAs = entry.roots
		config.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &entry.cert, nil
		}
	}
	return config
}

// handshake opens TLS to the server at host over raw, presenting entry's
// certificate when the server asks for one.
func (s *server) handshake(ctx context.Context, raw net.Conn, via route, host string, entry *upstreamEntry, protocols []string) (*upstreamConn, error) {
	requested := &atomic.Bool{}
	conn := tls.Client(raw, &tls.Config{
		ServerName: host,
		NextProtos: protocols,
		RootCAs:    entry.roots,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			requested.Store(true)
			// The guard's entry is empty, which sends no certificate.
			return &entry.cert, nil
		},
		Renegotiation:      tls.RenegotiateFreelyAsClient,
		MinVersion:         tls.VersionTLS12,
		ClientSessionCache: entry.sessions,
	})
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		s.reportHandshakeFailure(host, entry, requested.Load(), err)
		return nil, err
	}
	return &upstreamConn{Conn: conn, via: via, requested: requested}, nil
}

// reportConnected records the first connection an entry makes to host, or,
// for the guard, a server that asked for a certificate it does not have.
func (s *server) reportConnected(host string, entry *upstreamEntry, upstream *upstreamConn) {
	switch {
	case !entry.presents():
		if upstream.requested.Load() {
			s.reportMissing(host)
		}
	case upstream.requested.Load():
		s.events.connect(host, entry.label, true, fmt.Sprintf("presented the client certificate %s to %s through %s", entry.label, host, upstream.via))
	default:
		s.events.connect(host, entry.label, false, fmt.Sprintf("connected to %s through %s; it did not ask for a client certificate", host, upstream.via))
	}
}

// reportHandshakeFailure records why the handshake with a server failed.
func (s *server) reportHandshakeFailure(host string, entry *upstreamEntry, requested bool, err error) {
	if alert, ok := rejectionAlert(err); ok && requested {
		s.reportRefusal(host, entry, alert)
		return
	}
	if verifyErr, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		message := fmt.Sprintf("%s presented a server certificate that is not trusted (%v)", host, verifyErr.Err)
		if entry.presents() {
			message += fmt.Sprintf("; if a private CA issued it, name that CA in ca_file of %s", entry.label)
		}
		s.events.add(EventServerCertificate, host, entry.label, message)
		return
	}
	s.events.add(EventUpstream, host, entry.label, fmt.Sprintf("the TLS handshake with %s failed: %v", host, err))
}

// reportRefusal records a server that refused the browser for its client
// certificate: the one entry presented, or none at all.
func (s *server) reportRefusal(host string, entry *upstreamEntry, alert string) {
	if !entry.presents() {
		s.reportMissing(host)
		return
	}
	s.events.add(EventClientCertificateRejected, host, entry.label, fmt.Sprintf("%s rejected the client certificate %s: %s", host, entry.label, alert))
}

func (s *server) reportMissing(host string) {
	s.events.add(EventClientCertificateMissing, host, "", fmt.Sprintf("%s asks for a TLS client certificate, and none is configured for it", host))
}

// rejectionAlert returns the alert of err when it is one by which a server
// refuses a client certificate.
func rejectionAlert(err error) (string, bool) {
	var opErr *net.OpError
	if !errors.As(err, &opErr) || opErr.Op != remoteErrorOp || opErr.Err == nil {
		return "", false
	}
	for _, alert := range rejectionAlerts {
		if opErr.Err.Error() == alert.Error() {
			return strings.TrimPrefix(alert.Error(), alertPrefix), true
		}
	}
	return "", false
}

// relay copies between the browser and the server until either side ends.
func (s *server) relay(client *tls.Conn, upstream *upstreamConn, host string, entry *upstreamEntry, pending int64) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = client.Close()
			_ = upstream.Close()
		})
	}
	go func() {
		_, _ = io.Copy(upstream, client)
		closeBoth()
	}()
	s.copyFromServer(client, upstream, host, entry, pending)
	closeBoth()
}

// copyFromServer copies what the server sends to the browser. Under TLS
// 1.3, and in a TLS 1.2 renegotiation, a server refuses a client
// certificate after the handshake, so an alert that ends the connection
// before it sent anything is reported.
func (s *server) copyFromServer(client net.Conn, upstream *upstreamConn, host string, entry *upstreamEntry, pending int64) {
	buf := make([]byte, relayBufferBytes)
	received := false
	for {
		n, err := upstream.Read(buf)
		if n > 0 {
			if !received {
				received = true
				s.pending.end(pending)
			}
			if _, err := client.Write(buf[:n]); err != nil {
				return
			}
		}
		if err == nil {
			continue
		}
		if alert, ok := rejectionAlert(err); ok && !received && upstream.requested.Load() {
			s.reportRefusal(host, entry, alert)
		}
		return
	}
}

func (s *server) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.tunnels[conn] = struct{}{}
	return true
}

func (s *server) untrack(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tunnels, conn)
}

// closeTunnels ends every tunnel, and refuses new ones.
func (s *server) closeTunnels() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for conn := range s.tunnels {
		_ = conn.Close()
	}
}

// pacRequestURL returns the URL a browser hands a PAC script for an https:
// connection to host and port.
func pacRequestURL(host, port string) string {
	if port == defaultTLSPort {
		if addr, err := netip.ParseAddr(host); err == nil && addr.Is6() {
			return "https://[" + host + "]/"
		}
		return "https://" + host + "/"
	}
	return "https://" + net.JoinHostPort(host, port) + "/"
}

// oneLine joins the lines of err, which errors.Join separates.
func oneLine(err error) string {
	return strings.ReplaceAll(err.Error(), "\n", "; ")
}

// bufferedConn reads what was buffered from a connection before the rest.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// leafIssuer mints the certificates the browser is shown, one per host, all
// self-signed with one key, so that a single key hash covers them.
type leafIssuer struct {
	key *ecdsa.PrivateKey
	// spki is the base64 SHA-256 hash of the key's SubjectPublicKeyInfo.
	spki   string
	mu     sync.Mutex
	leaves map[string]*tls.Certificate
}

func newLeafIssuer() (*leafIssuer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(der)
	return &leafIssuer{key: key, spki: base64.StdEncoding.EncodeToString(sum[:]), leaves: map[string]*tls.Certificate{}}, nil
}

// leaf returns the certificate for host.
func (l *leafIssuer) leaf(host string) (*tls.Certificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if leaf, ok := l.leaves[host]; ok {
		return leaf, nil
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), leafSerialBits))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             now.Add(-leafBackdate),
		NotAfter:              now.Add(leafLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if len(host) <= maxCommonNameLength {
		template.Subject = pkix.Name{CommonName: host}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		template.IPAddresses = []net.IP{addr.AsSlice()}
	} else {
		template.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &l.key.PublicKey, l.key)
	if err != nil {
		return nil, fmt.Errorf("mint a certificate for %s: %w", host, err)
	}
	leaf := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: l.key}
	l.leaves[host] = leaf
	return leaf, nil
}
