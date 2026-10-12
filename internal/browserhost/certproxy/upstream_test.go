// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package certproxy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoServer accepts connections and answers each with "hello" once the
// client wrote a byte, as a TLS server speaks only after its client.
func echoServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				if _, err := conn.Read(make([]byte, 1)); err == nil {
					_, _ = conn.Write([]byte("hello"))
				}
			}()
		}
	}()
	return listener.Addr().String()
}

func readGreeting(t *testing.T, conn net.Conn) string {
	t.Helper()
	defer func() { _ = conn.Close() }()
	_, err := conn.Write([]byte("?"))
	require.NoError(t, err)
	greeting, err := io.ReadAll(conn)
	require.NoError(t, err)
	return string(greeting)
}

// noCertificate configures TLS to a proxy on the way without a client
// certificate.
func noCertificate(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

func TestDialRoutes(t *testing.T) {
	t.Parallel()

	target := echoServer(t)
	refusing := strings.TrimPrefix(closedURL(t), "http://")

	t.Run("FailsOverInOrder", func(t *testing.T) {
		t.Parallel()
		authProxy := newTunnelProxy(t, target, http.StatusProxyAuthRequired)
		routes := []route{
			{kind: routeProxy, addr: refusing},
			{kind: routeProxy, addr: authProxy.addr},
			{kind: routeDirect},
		}
		conn, via, err := dialRoutes(context.Background(), routes, target, noCertificate)
		require.NoError(t, err)
		assert.Equal(t, route{kind: routeDirect}, via)
		assert.Equal(t, "hello", readGreeting(t, conn))
		assert.Equal(t, []string{target}, authProxy.requests())
	})

	t.Run("TunnelsThroughProxy", func(t *testing.T) {
		t.Parallel()
		tunnel := newTunnelProxy(t, target, http.StatusOK)
		conn, via, err := dialRoutes(context.Background(), []route{{kind: routeProxy, addr: tunnel.addr}}, "portal.test:443", noCertificate)
		require.NoError(t, err)
		assert.Equal(t, routeProxy, via.kind)
		assert.Equal(t, "hello", readGreeting(t, conn))
		assert.Equal(t, []string{"portal.test:443"}, tunnel.requests())
	})

	t.Run("TunnelsThroughSOCKS5", func(t *testing.T) {
		t.Parallel()
		socks := newSOCKSProxy(t, target)
		conn, _, err := dialRoutes(context.Background(), []route{{kind: routeSOCKS5, addr: socks.addr}}, "portal.test:443", noCertificate)
		require.NoError(t, err)
		assert.Equal(t, "hello", readGreeting(t, conn))
		assert.Equal(t, []string{"portal.test:443"}, socks.requests())
	})

	t.Run("ReportsEveryRoute", func(t *testing.T) {
		t.Parallel()
		authProxy := newTunnelProxy(t, target, http.StatusProxyAuthRequired)
		_, _, err := dialRoutes(context.Background(), []route{
			{kind: routeProxy, addr: authProxy.addr},
			{kind: routeDirect},
		}, refusing, noCertificate)
		require.Error(t, err)
		assert.ErrorIs(t, err, errProxyAuth)
		message := oneLine(err)
		proxyAt, directAt := strings.Index(message, "through PROXY "), strings.Index(message, "through DIRECT")
		assert.True(t, proxyAt >= 0 && directAt > proxyAt, message)
	})

	t.Run("NoRoute", func(t *testing.T) {
		t.Parallel()
		_, _, err := dialRoutes(context.Background(), nil, target, noCertificate)
		assert.Error(t, err)
	})
}
