// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := LoadConfig(env(map[string]string{
		"IMAP_HOST":    "imap.example.net",
		"SMTP_HOST":    "smtp.example.net",
		"DISPLAY_NAME": "  ",
		"POP3_PORT":    "",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.IMAPHost, want.SMTPHost = "imap.example.net", "smtp.example.net"
	if c != want {
		t.Errorf("got %+v\nwant %+v", c, want)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	c, err := LoadConfig(env(map[string]string{
		"IMAP_HOST":              "imap.example.net",
		"IMAP_PORT":              "1993",
		"SMTP_HOST":              "smtp.example.net",
		"SMTP_IMPLICIT_TLS_PORT": "0",
		"SMTP_STARTTLS_PORT":     "2587",
		"POP3_HOST":              "pop.example.net",
		"POP3_PORT":              "1995",
		"DAV_URL":                "https://dav.example.net/",
		"JMAP_URL":               "https://jmap.example.net/.well-known/jmap",
		"DISPLAY_NAME":           "Example",
		"LISTEN_ADDR":            ":9000",
		"METRICS_ADDR":           ":9001",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		ListenAddr: ":9000", MetricsAddr: ":9001", DisplayName: "Example",
		IMAPHost: "imap.example.net", IMAPPort: 1993,
		SMTPHost: "smtp.example.net", SMTPImplicitTLSPort: 0, SMTPStartTLSPort: 2587,
		POP3Host: "pop.example.net", POP3Port: 1995,
		DAVURL: "https://dav.example.net/", JMAPURL: "https://jmap.example.net/.well-known/jmap",
	}
	if c != want {
		t.Errorf("got %+v\nwant %+v", c, want)
	}
}

func TestLoadConfigRefuses(t *testing.T) {
	base := map[string]string{"IMAP_HOST": "imap.example.net", "SMTP_HOST": "smtp.example.net"}
	tests := []struct {
		name string
		set  map[string]string
		msg  string
	}{
		{"no imap host", map[string]string{"IMAP_HOST": ""}, "IMAP_HOST"},
		{"bare imap host", map[string]string{"IMAP_HOST": "imap"}, "IMAP_HOST"},
		{"no smtp host", map[string]string{"SMTP_HOST": ""}, "SMTP_HOST"},
		{"port text", map[string]string{"IMAP_PORT": "imaps"}, "IMAP_PORT"},
		{"port negative", map[string]string{"IMAP_PORT": "-1"}, "IMAP_PORT"},
		{"port too big", map[string]string{"IMAP_PORT": "65536"}, "IMAP_PORT"},
		{"imap port zero", map[string]string{"IMAP_PORT": "0"}, "IMAP_PORT: 0"},
		{"both smtp ports zero", map[string]string{"SMTP_IMPLICIT_TLS_PORT": "0", "SMTP_STARTTLS_PORT": "0"}, "both 0"},
		{"pop3 host bad", map[string]string{"POP3_HOST": "pop_3.example.net"}, "POP3_HOST"},
		{"pop3 port zero", map[string]string{"POP3_HOST": "pop.example.net", "POP3_PORT": "0"}, "POP3_PORT: 0"},
		{"dav plain http", map[string]string{"DAV_URL": "http://dav.example.net/"}, "DAV_URL"},
		{"jmap no host", map[string]string{"JMAP_URL": "https:///jmap"}, "JMAP_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kv := map[string]string{}
			for k, v := range base {
				kv[k] = v
			}
			for k, v := range tt.set {
				kv[k] = v
			}
			_, err := LoadConfig(env(kv))
			if err == nil || !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("err = %v, want one naming %q", err, tt.msg)
			}
		})
	}
}

// TestValuesMatchDefaults loads helm/values.yaml through LoadConfig the way
// the chart renders it: every config key upper-cased into the environment.
func TestValuesMatchDefaults(t *testing.T) {
	raw, err := os.ReadFile("helm/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		Config map[string]any `yaml:"config"`
	}
	if err := yaml.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	kv := map[string]string{}
	for k, v := range values.Config {
		switch v := v.(type) {
		case int:
			kv[strings.ToUpper(k)] = strconv.Itoa(v)
		case string:
			kv[strings.ToUpper(k)] = v
		default:
			t.Fatalf("config.%s: unexpected type %T", k, v)
		}
	}
	for _, k := range []string{"IMAP_HOST", "SMTP_HOST"} {
		if kv[k] != "" {
			t.Errorf("values.yaml config.%s must stay empty (required)", strings.ToLower(k))
		}
		kv[k] = strings.ToLower(k[:4]) + ".example.net"
	}
	c, err := LoadConfig(env(kv))
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.IMAPHost, want.SMTPHost = "imap.example.net", "smtp.example.net"
	if c != want {
		t.Errorf("values.yaml differs from Defaults():\ngot  %+v\nwant %+v", c, want)
	}
	known := map[string]bool{"DISPLAY_NAME": true, "IMAP_HOST": true, "IMAP_PORT": true, "SMTP_HOST": true,
		"SMTP_IMPLICIT_TLS_PORT": true, "SMTP_STARTTLS_PORT": true, "POP3_HOST": true, "POP3_PORT": true,
		"DAV_URL": true, "JMAP_URL": true}
	for k := range kv {
		if !known[k] {
			t.Errorf("values.yaml config.%s is not read by LoadConfig", strings.ToLower(k))
		}
	}
	if len(kv) != len(known) {
		t.Errorf("values.yaml has %d config keys, LoadConfig reads %d", len(kv), len(known))
	}
}

func TestParseAddress(t *testing.T) {
	for addr, want := range map[string]string{
		"alice@Example.NET":       "example.net",
		"o'brien&co@example.net":  "example.net",
		"a+tag@sub.example.co.uk": "sub.example.co.uk",
	} {
		if got, ok := parseAddress(addr); !ok || got != want {
			t.Errorf("parseAddress(%q) = %q, %v", addr, got, ok)
		}
	}
	for _, addr := range []string{"", "alice", "@example.net", "alice@", "a@b@example.net",
		"alice@example", "alice@-x.example.net", "al ice@example.net", "<a>@example.net",
		strings.Repeat("a", 243) + "@example.net"} {
		if _, ok := parseAddress(addr); ok {
			t.Errorf("parseAddress(%q) accepted", addr)
		}
	}
}
