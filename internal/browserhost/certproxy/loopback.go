// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package certproxy

import (
	"net/netip"
	"strings"

	"github.com/dagucloud/dagu/v2/internal/cmn/hostpattern"
)

// manualProxy reports whether hosts include a destination Chrome will not
// send through a PAC script. The browser then needs a manual proxy.
func manualProxy(hosts []string) bool {
	for _, host := range hosts {
		if implicitBypass(host) {
			return true
		}
	}
	return false
}

// implicitBypass reports whether pattern is, or can match, a destination
// Chrome skips in every PAC script: localhost names, the Windows loopback
// name, *.localhost, *, a loopback address, or a link-local address.
func implicitBypass(pattern string) bool {
	pattern = hostpattern.Canonical(pattern)
	if pattern == hostpattern.Any || pattern == "localhost" || pattern == "loopback" || strings.HasSuffix(pattern, ".localhost") {
		return true
	}
	address, err := netip.ParseAddr(pattern)
	return err == nil && (address.Unmap().IsLoopback() || address.IsLinkLocalUnicast())
}
