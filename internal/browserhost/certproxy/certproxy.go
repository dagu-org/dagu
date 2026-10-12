// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package certproxy presents TLS client certificates on behalf of a browser.
// The browser sends its connections to the certificate hosts through a local
// proxy, which ends the browser's TLS with a certificate the browser is told
// to accept and opens TLS to the site with the client certificate, routed
// the way the device routes its own traffic.
//
//	browser ── PAC ──┬─ https: certificate host ── proxy ── TLS + client cert ── site
//	                 └─ anything else ───────────── the device's own proxy setup
//
// The proxy runs as a process of its own, because the browser outlives the
// process that launched it, and exits once that browser is gone.
package certproxy

import (
	"sync"
	"time"
)

// Entry is a client certificate ready for the proxy, and the hosts it is
// presented to.
type Entry struct {
	// Label names the entry in messages, such as
	// with.browser.client_certificates[0].
	Label string `json:"label"`
	// Hosts are host patterns, as the hostpattern package matches them.
	Hosts []string `json:"hosts"`
	// ChainPEM holds the certificate, then its intermediates.
	ChainPEM string `json:"chain_pem"`
	// KeyPEM holds the certificate's private key as PKCS#8.
	KeyPEM string `json:"key_pem"`
	// CAPEM holds certificates trusted, beside the system's, for the hosts'
	// servers.
	CAPEM string `json:"ca_pem,omitempty"`
}

// Config is what the proxy serves, and for which browser.
type Config struct {
	// Token is a random value that prefixes the proxy's control paths.
	Token string `json:"token"`
	// CDPURL is the DevTools address of the browser the proxy serves, such
	// as http://127.0.0.1:9333.
	CDPURL string `json:"cdp_url"`
	// SystemPAC is a PAC script for the device's own routing. Empty routes
	// everything directly.
	SystemPAC string `json:"system_pac"`
	// Entries are the certificates; the first whose hosts match a host is
	// presented to it.
	Entries []Entry `json:"entries"`
	// GraceSeconds is how long the proxy waits for the browser to answer
	// for the first time; zero means five minutes.
	GraceSeconds int `json:"grace_seconds,omitempty"`
}

// Ready describes a proxy that accepts connections.
type Ready struct {
	Port int `json:"port"`
	// SPKI is the base64 SHA-256 hash of the public key of every
	// certificate the proxy shows the browser, in the form Chrome's
	// --ignore-certificate-errors-spki-list takes.
	SPKI string `json:"spki"`
	// PACURL is the PAC script the browser uses: a data: URL, or one the
	// proxy serves when the script is too large for a command line.
	PACURL string `json:"pac_url"`
}

// Kinds of Event.
const (
	// EventConnect reports the first connection to a host with an entry.
	EventConnect = "connect"
	// EventClientCertificateRejected reports a server that refused the
	// client certificate.
	EventClientCertificateRejected = "client_certificate_rejected"
	// EventServerCertificate reports a server whose certificate is not
	// trusted.
	EventServerCertificate = "server_certificate"
	// EventProxyAuth reports an upstream proxy that asks for credentials.
	EventProxyAuth = "proxy_auth"
	// EventUpstream reports a server the proxy could not reach.
	EventUpstream = "upstream"
	// EventRefused reports a connection to a host no entry names.
	EventRefused = "refused"
)

// Event is something the proxy reports about a connection.
type Event struct {
	Seq  int64     `json:"seq"`
	Time time.Time `json:"time"`
	Kind string    `json:"kind"`
	Host string    `json:"host"`
	// Entry is the label of the entry involved, if any.
	Entry string `json:"entry,omitempty"`
	// Message explains the event in a sentence of its own.
	Message string `json:"message,omitempty"`
}

const (
	// maxEvents bounds the events kept for readers.
	maxEvents = 200
	// repeatWindow is how long an event repeated by the browser's retries
	// is reported once.
	repeatWindow = 10 * time.Second
)

// eventLog keeps the latest events, without repeats.
type eventLog struct {
	mu     sync.Mutex
	seq    int64
	events []Event
	// recent holds when each event was last reported.
	recent map[eventKey]time.Time
	// connected holds the hosts each entry connected to.
	connected map[connectKey]bool
}

type eventKey struct {
	kind, host, message string
}

type connectKey struct {
	entry, host string
}

func newEventLog() *eventLog {
	return &eventLog{recent: map[eventKey]time.Time{}, connected: map[connectKey]bool{}}
}

// add records an event unless the same one was recorded within
// repeatWindow.
func (l *eventLog) add(kind, host, entry, message string) {
	now := time.Now()
	key := eventKey{kind: kind, host: host, message: message}
	l.mu.Lock()
	defer l.mu.Unlock()
	if last, ok := l.recent[key]; ok && now.Sub(last) < repeatWindow {
		return
	}
	for k, last := range l.recent {
		if now.Sub(last) >= repeatWindow {
			delete(l.recent, k)
		}
	}
	l.recent[key] = now
	l.seq++
	l.events = append(l.events, Event{Seq: l.seq, Time: now, Kind: kind, Host: host, Entry: entry, Message: message})
	if len(l.events) > maxEvents {
		l.events = append(l.events[:0:0], l.events[len(l.events)-maxEvents:]...)
	}
}

// connect records the first connection entry makes to host.
func (l *eventLog) connect(host, entry, message string) {
	key := connectKey{entry: entry, host: host}
	l.mu.Lock()
	seen := l.connected[key]
	l.connected[key] = true
	l.mu.Unlock()
	if !seen {
		l.add(EventConnect, host, entry, message)
	}
}

// after returns the events recorded after seq.
func (l *eventLog) after(seq int64) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	events := []Event{}
	for _, event := range l.events {
		if event.Seq > seq {
			events = append(events, event)
		}
	}
	return events
}
