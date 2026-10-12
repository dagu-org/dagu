// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package certproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

const (
	// dialTimeout bounds each attempt to reach a server or proxy.
	dialTimeout = 15 * time.Second
	// maxProxyResponseBytes bounds the header of a proxy's CONNECT answer.
	maxProxyResponseBytes = 64 << 10
)

// PAC result keywords.
const (
	routeKeywordDirect = "DIRECT"
	routeKeywordProxy  = "PROXY"
	routeKeywordHTTPS  = "HTTPS"
	routeKeywordSOCKS5 = "SOCKS5"
)

// errProxyAuth reports an upstream proxy that asks for credentials.
var errProxyAuth = errors.New("asks for credentials, and proxies that need them are not supported for certificate hosts")

type routeKind int

const (
	routeDirect routeKind = iota
	// routeProxy tunnels through an HTTP proxy with CONNECT.
	routeProxy
	// routeHTTPS tunnels through an HTTP proxy reached over TLS.
	routeHTTPS
	routeSOCKS5
)

// route is one way to reach a server.
type route struct {
	kind routeKind
	// addr is the proxy's host and port.
	addr string
}

func (r route) String() string {
	switch r.kind {
	case routeProxy:
		return routeKeywordProxy + " " + r.addr
	case routeHTTPS:
		return routeKeywordHTTPS + " " + r.addr
	case routeSOCKS5:
		return routeKeywordSOCKS5 + " " + r.addr
	case routeDirect:
	}
	return routeKeywordDirect
}

// dialRoutes connects to target through the first of routes that reaches
// it. proxyTLS returns the TLS configuration for an HTTPS proxy on the way.
func dialRoutes(ctx context.Context, routes []route, target string, proxyTLS func(host string) *tls.Config) (net.Conn, route, error) {
	var errs []error
	for _, r := range routes {
		conn, err := dialRoute(ctx, r, target, proxyTLS)
		if err == nil {
			return conn, r, nil
		}
		errs = append(errs, fmt.Errorf("through %s: %w", r, err))
		if ctx.Err() != nil {
			break
		}
	}
	if len(errs) == 0 {
		return nil, route{}, errors.New("the device's proxy settings give it no route the certificate proxy supports")
	}
	return nil, route{}, errors.Join(errs...)
}

func dialRoute(ctx context.Context, r route, target string, proxyTLS func(host string) *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: dialTimeout}
	switch r.kind {
	case routeProxy, routeHTTPS:
		conn, err := dialer.DialContext(ctx, "tcp", r.addr)
		if err != nil {
			return nil, err
		}
		if r.kind == routeHTTPS {
			host, _, _ := net.SplitHostPort(r.addr)
			tlsConn := tls.Client(conn, proxyTLS(host))
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				_ = conn.Close()
				return nil, err
			}
			conn = tlsConn
		}
		if err := connectTunnel(ctx, conn, r.addr, target); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	case routeSOCKS5:
		socks, err := proxy.SOCKS5("tcp", r.addr, nil, dialer)
		if err != nil {
			return nil, err
		}
		return socks.(proxy.ContextDialer).DialContext(ctx, "tcp", target)
	case routeDirect:
	}
	return dialer.DialContext(ctx, "tcp", target)
}

// connectTunnel asks the HTTP proxy at proxyAddr, reached over conn, for a
// tunnel to target.
func connectTunnel(ctx context.Context, conn net.Conn, proxyAddr, target string) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(dialTimeout)
	}
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	status, err := requestTunnel(conn, target)
	if !stop() && err == nil {
		err = ctx.Err()
	}
	switch {
	case err != nil:
		return err
	case status == http.StatusProxyAuthRequired:
		return fmt.Errorf("the proxy %s %w", proxyAddr, errProxyAuth)
	case status != http.StatusOK:
		return fmt.Errorf("the proxy %s answered %d %s", proxyAddr, status, http.StatusText(status))
	}
	return conn.SetDeadline(time.Time{})
}

// requestTunnel sends a CONNECT request for target over conn and returns
// the status of the answer.
func requestTunnel(conn net.Conn, target string) (int, error) {
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
		return 0, err
	}
	// A server behind the tunnel speaks only after the TLS client does, so
	// nothing past the answer is buffered. The body of a refusal is left
	// unread; the caller closes the connection instead.
	response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(conn, maxProxyResponseBytes)), &http.Request{Method: http.MethodConnect}) //nolint:bodyclose // see above
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}
