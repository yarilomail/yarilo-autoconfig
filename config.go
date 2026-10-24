// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Config holds the endpoint hosts every answer points at.
type Config struct {
	ListenAddr          string
	MetricsAddr         string
	DisplayName         string
	IMAPHost            string
	IMAPPort            int
	SMTPHost            string
	SMTPImplicitTLSPort int
	SMTPStartTLSPort    int
	POP3Host            string
	POP3Port            int
	DAVURL              string
	JMAPURL             string
}

// Defaults returns the values used for every variable that is not set.
// helm/values.yaml carries the same numbers.
func Defaults() Config {
	return Config{
		ListenAddr:          ":8080",
		MetricsAddr:         ":8081",
		DisplayName:         "Mail",
		IMAPPort:            993,
		SMTPImplicitTLSPort: 465,
		SMTPStartTLSPort:    587,
		POP3Port:            995,
	}
}

// LoadConfig reads the configuration from getenv and validates it.
func LoadConfig(getenv func(string) string) (Config, error) {
	c := Defaults()
	str := func(dst *string, key string) {
		if v, ok := lookup(getenv, key); ok {
			*dst = v
		}
	}
	var errs []string
	num := func(dst *int, key string) {
		v, ok := lookup(getenv, key)
		if !ok {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 65535 {
			errs = append(errs, fmt.Sprintf("%s: not a port: %q", key, v))
			return
		}
		*dst = n
	}
	str(&c.ListenAddr, "LISTEN_ADDR")
	str(&c.MetricsAddr, "METRICS_ADDR")
	str(&c.DisplayName, "DISPLAY_NAME")
	str(&c.IMAPHost, "IMAP_HOST")
	num(&c.IMAPPort, "IMAP_PORT")
	str(&c.SMTPHost, "SMTP_HOST")
	num(&c.SMTPImplicitTLSPort, "SMTP_IMPLICIT_TLS_PORT")
	num(&c.SMTPStartTLSPort, "SMTP_STARTTLS_PORT")
	str(&c.POP3Host, "POP3_HOST")
	num(&c.POP3Port, "POP3_PORT")
	str(&c.DAVURL, "DAV_URL")
	str(&c.JMAPURL, "JMAP_URL")

	if c.DisplayName == "" {
		errs = append(errs, "DISPLAY_NAME: empty")
	}
	if !validHostname(c.IMAPHost) {
		errs = append(errs, fmt.Sprintf("IMAP_HOST: not a hostname: %q", c.IMAPHost))
	}
	if c.IMAPPort == 0 {
		errs = append(errs, "IMAP_PORT: 0")
	}
	if !validHostname(c.SMTPHost) {
		errs = append(errs, fmt.Sprintf("SMTP_HOST: not a hostname: %q", c.SMTPHost))
	}
	if c.SMTPImplicitTLSPort == 0 && c.SMTPStartTLSPort == 0 {
		errs = append(errs, "SMTP_IMPLICIT_TLS_PORT and SMTP_STARTTLS_PORT: both 0")
	}
	if c.POP3Host != "" {
		if !validHostname(c.POP3Host) {
			errs = append(errs, fmt.Sprintf("POP3_HOST: not a hostname: %q", c.POP3Host))
		}
		if c.POP3Port == 0 {
			errs = append(errs, "POP3_PORT: 0")
		}
	}
	for _, u := range []struct{ key, val string }{{"DAV_URL", c.DAVURL}, {"JMAP_URL", c.JMAPURL}} {
		if u.val != "" && !validHTTPSURL(u.val) {
			errs = append(errs, fmt.Sprintf("%s: not an https URL: %q", u.key, u.val))
		}
	}
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config: %s", strings.Join(errs, "; "))
	}
	return c, nil
}

// lookup treats a variable set to the empty string as unset, so a chart value
// left empty keeps the default.
func lookup(getenv func(string) string, key string) (string, bool) {
	v := strings.TrimSpace(getenv(key))
	return v, v != ""
}

func validHTTPSURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && validHostname(u.Hostname())
}

// validHostname accepts a fully qualified name of LDH labels (RFC 1123).
func validHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 || !strings.Contains(h, ".") {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
