// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// parseTrusted reads TRUSTED_PROXIES: CIDR ranges or single addresses,
// separated by commas or spaces.
func parseTrusted(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if p, err := netip.ParsePrefix(f); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(f)
		if err != nil {
			return nil, fmt.Errorf("not an address or CIDR range: %q", f)
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

func trustedAddr(a netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// hopAddr reads one hop of X-Forwarded-For or a Forwarded for= value: an
// address, quoted, bracketed, with or without a port.
func hopAddr(s string) (netip.Addr, bool) {
	s = strings.Trim(strings.TrimSpace(s), `"`)
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap(), true
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

// forwardedFor lists the for= values of the Forwarded header (RFC 7239), in
// order, across every Forwarded field.
func forwardedFor(h http.Header) []string {
	var out []string
	for _, field := range h.Values("Forwarded") {
		for _, elem := range strings.Split(field, ",") {
			for _, pair := range strings.Split(elem, ";") {
				k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
				if ok && strings.EqualFold(strings.TrimSpace(k), "for") {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// Forwarded-header choices (FORWARDED_HEADER): only the one the trusted
// proxies maintain is read, since the other passes through them as the client
// wrote it.
const (
	headerXFF       = "x-forwarded-for"
	headerForwarded = "forwarded"
)

// clientAddr is the address of the client a request came from. The TCP peer
// is it, unless the peer is a trusted proxy: then the chain in the header is
// walked from the right, past every trusted hop, and the first hop that is
// not trusted is the client. A client cannot name its own address: what it
// writes sits left of the hops the trusted proxies added. A hop that does not
// parse ends the walk at the last address known.
func clientAddr(r *http.Request, trusted []netip.Prefix, header string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	cur := peer.Unmap()
	if len(trusted) == 0 || !trustedAddr(cur, trusted) {
		return cur.String()
	}
	var hops []string
	if header == headerForwarded {
		hops = forwardedFor(r.Header)
	} else {
		for _, field := range r.Header.Values("X-Forwarded-For") {
			hops = append(hops, strings.Split(field, ",")...)
		}
	}
	for i := len(hops) - 1; i >= 0; i-- {
		a, ok := hopAddr(hops[i])
		if !ok {
			break
		}
		cur = a
		if !trustedAddr(a, trusted) {
			break
		}
	}
	return cur.String()
}
