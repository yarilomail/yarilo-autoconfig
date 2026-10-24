// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden")

func testConfig() Config {
	c := Defaults()
	c.DisplayName = "Example Mail"
	c.IMAPHost = "imap.example.net"
	c.SMTPHost = "smtp.example.net"
	return c
}

func newTestServer(t *testing.T, c Config) *server {
	t.Helper()
	s := newServer(c, prometheus.NewRegistry())
	s.now = func() time.Time { return time.Date(2026, 10, 24, 13, 7, 9, 123456700, time.UTC) }
	return s
}

func do(s *server, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

func poxBody(addr, schema string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006">
  <Request>
    <EMailAddress>` + addr + `</EMailAddress>
    <AcceptableResponseSchema>` + schema + `</AcceptableResponseSchema>
  </Request>
</Autodiscover>`
}

func TestAutoconfigGolden(t *testing.T) {
	pop3StartTLS := testConfig()
	pop3StartTLS.POP3Host = "pop.example.net"
	pop3StartTLS.SMTPImplicitTLSPort = 0

	tests := []struct {
		name, target string
		cfg          Config
	}{
		{"autoconfig_with_address", "/mail/config-v1.1.xml?emailaddress=alice%40Customer.Example", testConfig()},
		{"autoconfig_without_address", "/.well-known/autoconfig/mail/config-v1.1.xml", testConfig()},
		{"autoconfig_pop3_starttls_only", "/mail/config-v1.1.xml?emailaddress=bob%40customer.example", pop3StartTLS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(newTestServer(t, tt.cfg), http.MethodGet, tt.target, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/xml; charset=utf-8" {
				t.Errorf("Content-Type %q", ct)
			}
			golden(t, tt.name, rec.Body.Bytes())
		})
	}
}

func TestAutoconfigRejects(t *testing.T) {
	s := newTestServer(t, testConfig())
	for _, target := range []string{
		"/mail/config-v1.1.xml?emailaddress=no-at-sign",
		"/mail/config-v1.1.xml?emailaddress=a%40b%40example.net",
		"/mail/config-v1.1.xml?emailaddress=alice%40localhost",
		"/mail/config-v1.1.xml?emailaddress=%3Cx%3E%40example.net",
		"/mail/config-v1.1.xml?emailaddress=" + strings.Repeat("a", 250) + "%40example.net",
	} {
		if rec := do(s, http.MethodGet, target, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
	if rec := do(s, http.MethodPost, "/mail/config-v1.1.xml", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d, want 405", rec.Code)
	}
}

func TestAutodiscoverGolden(t *testing.T) {
	startTLS := testConfig()
	startTLS.SMTPImplicitTLSPort = 0
	startTLS.POP3Host = "pop.example.net"

	tests := []struct {
		name, path, body string
		cfg              Config
	}{
		{"autodiscover_settings", "/autodiscover/autodiscover.xml", poxBody("alice@customer.example", nsOutlook), testConfig()},
		{"autodiscover_settings", "/Autodiscover/Autodiscover.xml", poxBody("alice@customer.example", nsOutlook), testConfig()},
		{"autodiscover_starttls_pop3", "/autodiscover/autodiscover.xml", poxBody("bob@customer.example", nsOutlook), startTLS},
		{"autodiscover_escaped", "/autodiscover/autodiscover.xml", poxBody("o'brien&amp;co@customer.example", nsOutlook), testConfig()},
		{"autodiscover_error", "/autodiscover/autodiscover.xml",
			poxBody("alice@customer.example", "http://schemas.microsoft.com/exchange/autodiscover/mobilesync/responseschema/2006"), testConfig()},
		{"autodiscover_error", "/autodiscover/autodiscover.xml", poxBody("not-an-address", nsOutlook), testConfig()},
		{"autodiscover_error", "/autodiscover/autodiscover.xml", "<Autodiscover><Request>", testConfig()},
		{"autodiscover_error", "/autodiscover/autodiscover.xml",
			strings.Replace(poxBody("alice@customer.example", nsOutlook), nsRequest, "urn:other", 1), testConfig()},
	}
	for _, tt := range tests {
		t.Run(tt.name+tt.path, func(t *testing.T) {
			rec := do(newTestServer(t, tt.cfg), http.MethodPost, tt.path, tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			golden(t, tt.name, rec.Body.Bytes())
		})
	}
}

func TestAutodiscoverStatus(t *testing.T) {
	s := newTestServer(t, testConfig())
	tests := []struct {
		name, method, body, allow string
		want                      int
	}{
		{"get", http.MethodGet, "", http.MethodPost, http.StatusMethodNotAllowed},
		{"oversized body", http.MethodPost,
			poxBody(strings.Repeat("a", maxRequestBody)+"@customer.example", nsOutlook), "", http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		rec := do(s, tt.method, "/autodiscover/autodiscover.xml", tt.body)
		if rec.Code != tt.want || rec.Header().Get("Allow") != tt.allow {
			t.Errorf("%s: status %d Allow %q", tt.name, rec.Code, rec.Header().Get("Allow"))
		}
	}
}

func TestWellKnown(t *testing.T) {
	c := testConfig()
	c.DAVURL = "https://dav.example.net/dav/"
	s := newTestServer(t, c)
	for path, want := range map[string]string{
		"/.well-known/caldav":  "https://dav.example.net/dav/",
		"/.well-known/carddav": "https://dav.example.net/dav/",
	} {
		rec := do(s, http.MethodGet, path, "")
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != want {
			t.Errorf("%s: status %d Location %q", path, rec.Code, rec.Header().Get("Location"))
		}
	}
	for _, path := range []string{"/.well-known/jmap", "/", "/autodiscover"} {
		if rec := do(s, http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

func TestRequestCounter(t *testing.T) {
	s := newTestServer(t, testConfig())
	do(s, http.MethodGet, "/mail/config-v1.1.xml", "")
	do(s, http.MethodGet, "/mail/config-v1.1.xml?emailaddress=bad", "")
	do(s, http.MethodPost, "/autodiscover/autodiscover.xml", poxBody("alice@customer.example", nsOutlook))
	for _, c := range []struct {
		endpoint, status string
		want             float64
	}{
		{epAutoconfig, "200", 1},
		{epAutoconfig, "400", 1},
		{epAutodiscover, "200", 1},
	} {
		if got := testutil.ToFloat64(s.requests.WithLabelValues(c.endpoint, c.status)); got != c.want {
			t.Errorf("%s/%s = %v, want %v", c.endpoint, c.status, got, c.want)
		}
	}
}

func TestOpsHandler(t *testing.T) {
	reg := prometheus.NewRegistry()
	s := newServer(testConfig(), reg)
	do(s, http.MethodGet, "/mail/config-v1.1.xml", "")
	h := opsHandler(reg)
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), `autoconfig_requests_total{endpoint="autoconfig",status="200"} 1`) {
		t.Errorf("/metrics lacks the request counter:\n%s", rec.Body.String())
	}
}
