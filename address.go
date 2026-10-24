// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import "strings"

// maxAddressLen is the RFC 5321 path limit minus the angle brackets.
const maxAddressLen = 254

// parseAddress checks the shape of a client-supplied address and returns its
// domain in lower case. The address is never looked up.
func parseAddress(addr string) (domain string, ok bool) {
	if len(addr) == 0 || len(addr) > maxAddressLen || strings.Count(addr, "@") != 1 {
		return "", false
	}
	local, dom, _ := strings.Cut(addr, "@")
	if local == "" || strings.ContainsAny(local, " \t\r\n<>\"") || !validHostname(dom) {
		return "", false
	}
	return strings.ToLower(dom), true
}
