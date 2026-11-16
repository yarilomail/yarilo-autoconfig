// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// The keptemail path: client, Cloudflare, Traefik in the pod network.
const trustedTest = "10.42.0.0/24, fd42::/64, 173.245.48.0/20"

func TestClientAddr(t *testing.T) {
	trusted, err := parseTrusted(trustedTest)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, peer, header string
		xff, fwd           []string
		trust              bool
		want               string
	}{
		{"no trust: the peer", "10.42.0.7:4000", headerXFF, []string{"91.216.213.227"}, nil, false, "10.42.0.7"},
		{"untrusted peer: its header ignored", "198.51.100.9:4000", headerXFF, []string{"91.216.213.227"}, nil, true, "198.51.100.9"},
		{"through Traefik", "10.42.0.7:4000", headerXFF, []string{"91.216.213.227"}, nil, true, "91.216.213.227"},
		{"through Cloudflare and Traefik", "10.42.0.7:4000", headerXFF, []string{"91.216.213.227, 173.245.48.10"}, nil, true, "91.216.213.227"},
		{"a client naming itself", "10.42.0.7:4000", headerXFF, []string{"1.2.3.4, 91.216.213.227"}, nil, true, "91.216.213.227"},
		{"a client naming a trusted range", "10.42.0.7:4000", headerXFF, []string{"10.42.0.9, 91.216.213.227"}, nil, true, "91.216.213.227"},
		{"two header fields", "10.42.0.7:4000", headerXFF, []string{"1.2.3.4", "91.216.213.227"}, nil, true, "91.216.213.227"},
		{"IPv6 client", "[fd42::5]:4000", headerXFF, []string{"2001:41d0:404:200::1a55"}, nil, true, "2001:41d0:404:200::1a55"},
		{"only trusted hops: the leftmost", "10.42.0.7:4000", headerXFF, []string{"173.245.48.10"}, nil, true, "173.245.48.10"},
		{"no header: the peer", "10.42.0.7:4000", headerXFF, nil, nil, true, "10.42.0.7"},
		{"a hop that does not parse", "10.42.0.7:4000", headerXFF, []string{"91.216.213.227, unknown"}, nil, true, "10.42.0.7"},
		{"mapped IPv4 peer", "[::ffff:10.42.0.7]:4000", headerXFF, []string{"91.216.213.227"}, nil, true, "91.216.213.227"},
		{"Forwarded read when chosen", "10.42.0.7:4000", headerForwarded, nil, []string{`for="[2001:41d0:404:200::1a55]:4711";proto=https`}, true, "2001:41d0:404:200::1a55"},
		{"Forwarded ignored by default", "10.42.0.7:4000", headerXFF, []string{"91.216.213.227"}, []string{"for=1.2.3.4"}, true, "91.216.213.227"},
		{"XFF ignored with Forwarded chosen", "10.42.0.7:4000", headerForwarded, []string{"1.2.3.4"}, []string{"for=91.216.213.227"}, true, "91.216.213.227"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/mail/config-v1.1.xml", nil)
			r.RemoteAddr = tc.peer
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			for _, v := range tc.fwd {
				r.Header.Add("Forwarded", v)
			}
			tr := trusted
			if !tc.trust {
				tr = nil
			}
			if got := clientAddr(r, tr, tc.header); got != tc.want {
				t.Errorf("client %q, want %q", got, tc.want)
			}
		})
	}
}

// Each answered request is one log line with the client, the method, the
// path without its query (it may hold the mail address) and the status.
func TestRequestLogLine(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cfg := Defaults()
	cfg.IMAPHost, cfg.SMTPHost, cfg.TrustedProxies = "imap.example.net", "smtp.example.net", trustedTest
	s := newServer(cfg, prometheus.NewRegistry())
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/mail/config-v1.1.xml?emailaddress=alice@example.org", nil)
	r.RemoteAddr = "10.42.0.7:4000"
	r.Header.Set("X-Forwarded-For", "91.216.213.227, 173.245.48.10")
	s.ServeHTTP(httptest.NewRecorder(), r)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("%v: %s", err, buf.String())
	}
	want := map[string]any{"msg": "request", "client": "91.216.213.227", "method": "GET", "path": "/mail/config-v1.1.xml", "status": float64(200)}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("%s = %v, want %v (%s)", k, line[k], v, buf.String())
		}
	}
	if bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Errorf("the mail address reached the log: %s", buf.String())
	}
}
