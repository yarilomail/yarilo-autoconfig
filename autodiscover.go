// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Outlook POX Autodiscover (MS-OXDSCLI).
const (
	nsRequest         = "http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006"
	nsResponse        = "http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006"
	nsOutlook         = "http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a"
	maxRequestBody    = 8 << 10
	errInvalidRequest = "600"
)

type poxRequest struct {
	XMLName  xml.Name `xml:"Autodiscover"`
	Email    string   `xml:"Request>EMailAddress"`
	Accepted string   `xml:"Request>AcceptableResponseSchema"`
}

type poxResponse struct {
	XMLName  xml.Name `xml:"Autodiscover"`
	NS       string   `xml:"xmlns,attr"`
	Response any      `xml:"Response"`
}

type poxSettings struct {
	NS          string     `xml:"xmlns,attr"`
	DisplayName string     `xml:"User>DisplayName"`
	AccountType string     `xml:"Account>AccountType"`
	Action      string     `xml:"Account>Action"`
	Protocols   []protocol `xml:"Account>Protocol"`
}

type protocol struct {
	Type           string `xml:"Type"`
	Server         string `xml:"Server"`
	Port           int    `xml:"Port"`
	DomainRequired string `xml:"DomainRequired"`
	LoginName      string `xml:"LoginName"`
	SPA            string `xml:"SPA"`
	SSL            string `xml:"SSL"`
	Encryption     string `xml:"Encryption"`
	AuthRequired   string `xml:"AuthRequired"`
}

type poxError struct {
	Error poxErrorBody `xml:"Error"`
}

type poxErrorBody struct {
	Time      string   `xml:"Time,attr"`
	ID        string   `xml:"Id,attr"`
	ErrorCode string   `xml:"ErrorCode"`
	Message   string   `xml:"Message"`
	DebugData struct{} `xml:"DebugData"`
}

func buildPOXSettings(c Config, addr string) poxResponse {
	proto := func(typ, host string, port int, enc string) protocol {
		return protocol{
			Type:           typ,
			Server:         host,
			Port:           port,
			DomainRequired: "off",
			LoginName:      addr,
			SPA:            "off",
			SSL:            "on",
			Encryption:     enc,
			AuthRequired:   "on",
		}
	}
	ps := []protocol{proto("IMAP", c.IMAPHost, c.IMAPPort, "SSL")}
	if c.POP3Host != "" {
		ps = append(ps, proto("POP3", c.POP3Host, c.POP3Port, "SSL"))
	}
	// Outlook takes one SMTP block; implicit TLS wins when both are on.
	if c.SMTPImplicitTLSPort != 0 {
		ps = append(ps, proto("SMTP", c.SMTPHost, c.SMTPImplicitTLSPort, "SSL"))
	} else {
		ps = append(ps, proto("SMTP", c.SMTPHost, c.SMTPStartTLSPort, "TLS"))
	}
	return poxResponse{NS: nsResponse, Response: poxSettings{
		NS:          nsOutlook,
		DisplayName: c.DisplayName,
		AccountType: "email",
		Action:      "settings",
		Protocols:   ps,
	}}
}

func (s *server) poxErrorResponse() poxResponse {
	return poxResponse{NS: nsResponse, Response: poxError{Error: poxErrorBody{
		Time:      s.now().UTC().Format("15:04:05.0000000"),
		ID:        "1",
		ErrorCode: errInvalidRequest,
		Message:   "Invalid Request",
	}}}
}

// handleAutodiscover serves POST /autodiscover/autodiscover.xml. Anything it
// cannot answer, ActiveSync requests included, gets the POX error with 200.
func (s *server) handleAutodiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		s.reply(w, epAutodiscover, http.StatusMethodNotAllowed, "", nil)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		s.reply(w, epAutodiscover, http.StatusRequestEntityTooLarge, "", nil)
		return
	}
	var req poxRequest
	if err != nil || xml.Unmarshal(body, &req) != nil || req.XMLName.Space != nsRequest ||
		!strings.EqualFold(strings.TrimSpace(req.Accepted), nsOutlook) {
		s.reply(w, epAutodiscover, http.StatusOK, "application/xml", s.poxErrorResponse())
		return
	}
	addr := strings.TrimSpace(req.Email)
	domain, ok := parseAddress(addr)
	if !ok {
		s.reply(w, epAutodiscover, http.StatusOK, "application/xml", s.poxErrorResponse())
		return
	}
	debugf("autodiscover", "domain", domain)
	s.reply(w, epAutodiscover, http.StatusOK, "application/xml", buildPOXSettings(s.cfg, addr))
}
