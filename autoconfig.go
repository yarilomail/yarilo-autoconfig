// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/xml"
	"net/http"
)

// Thunderbird autoconfig, format version 1.1.

type clientConfig struct {
	XMLName  xml.Name      `xml:"clientConfig"`
	Version  string        `xml:"version,attr"`
	Provider emailProvider `xml:"emailProvider"`
}

type emailProvider struct {
	ID               string       `xml:"id,attr"`
	Domain           string       `xml:"domain"`
	DisplayName      string       `xml:"displayName"`
	DisplayShortName string       `xml:"displayShortName"`
	Incoming         []mailServer `xml:"incomingServer"`
	Outgoing         []mailServer `xml:"outgoingServer"`
}

type mailServer struct {
	Type           string `xml:"type,attr"`
	Hostname       string `xml:"hostname"`
	Port           int    `xml:"port"`
	SocketType     string `xml:"socketType"`
	Authentication string `xml:"authentication"`
	Username       string `xml:"username"`
}

// buildClientConfig answers for domain; the username stays the
// %EMAILADDRESS% placeholder the client fills in itself.
func buildClientConfig(c Config, domain string) clientConfig {
	server := func(typ, host string, port int, socket string) mailServer {
		return mailServer{
			Type:           typ,
			Hostname:       host,
			Port:           port,
			SocketType:     socket,
			Authentication: "password-cleartext",
			Username:       "%EMAILADDRESS%",
		}
	}
	p := emailProvider{
		ID:               domain,
		Domain:           domain,
		DisplayName:      c.DisplayName,
		DisplayShortName: c.DisplayName,
		Incoming:         []mailServer{server("imap", c.IMAPHost, c.IMAPPort, "SSL")},
	}
	if c.POP3Host != "" {
		p.Incoming = append(p.Incoming, server("pop3", c.POP3Host, c.POP3Port, "SSL"))
	}
	// Implicit TLS before STARTTLS (RFC 8314 §4).
	if c.SMTPImplicitTLSPort != 0 {
		p.Outgoing = append(p.Outgoing, server("smtp", c.SMTPHost, c.SMTPImplicitTLSPort, "SSL"))
	}
	if c.SMTPStartTLSPort != 0 {
		p.Outgoing = append(p.Outgoing, server("smtp", c.SMTPHost, c.SMTPStartTLSPort, "STARTTLS"))
	}
	return clientConfig{Version: "1.1", Provider: p}
}

// handleAutoconfig serves GET config-v1.1.xml. The address is optional: the
// .well-known URL is fetched without it.
func (s *server) handleAutoconfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.reply(w, epAutoconfig, http.StatusMethodNotAllowed, "", nil)
		return
	}
	domain := hostDomain(s.cfg.IMAPHost)
	if addr := r.URL.Query().Get("emailaddress"); addr != "" {
		d, ok := parseAddress(addr)
		if !ok {
			s.reply(w, epAutoconfig, http.StatusBadRequest, "", nil)
			return
		}
		domain = d
	}
	debugf("autoconfig", "domain", domain)
	s.reply(w, epAutoconfig, http.StatusOK, "application/xml", buildClientConfig(s.cfg, domain))
}

// hostDomain drops the first label: imap.example.net -> example.net.
func hostDomain(host string) string {
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			return host[i+1:]
		}
	}
	return host
}
