// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/xml"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Endpoint labels for the request counter.
const (
	epAutoconfig   = "autoconfig"
	epAutodiscover = "autodiscover"
	epWellKnown    = "well_known"
	epOther        = "other"
)

type server struct {
	cfg      Config
	now      func() time.Time
	requests *prometheus.CounterVec
}

func newServer(cfg Config, reg prometheus.Registerer) *server {
	s := &server{
		cfg: cfg,
		now: time.Now,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "autoconfig_requests_total",
			Help: "Requests by endpoint and HTTP status.",
		}, []string{"endpoint", "status"}),
	}
	reg.MustRegister(s.requests)
	return s
}

// ServeHTTP routes by lower-cased path: Outlook sends both
// /autodiscover/autodiscover.xml and /Autodiscover/Autodiscover.xml.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A browser must never sniff an XML answer into HTML.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch strings.ToLower(r.URL.Path) {
	case "/mail/config-v1.1.xml", "/.well-known/autoconfig/mail/config-v1.1.xml":
		s.handleAutoconfig(w, r)
	case "/autodiscover/autodiscover.xml":
		s.handleAutodiscover(w, r)
	case "/.well-known/caldav", "/.well-known/carddav":
		s.redirect(w, r, s.cfg.DAVURL)
	case "/.well-known/jmap":
		s.redirect(w, r, s.cfg.JMAPURL)
	default:
		s.reply(w, epOther, http.StatusNotFound, "", nil)
	}
}

// redirect sends a user who typed the autoconfig host into a DAV or JMAP
// client on to the real service; discovery itself is answered there.
func (s *server) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if target == "" {
		s.reply(w, epWellKnown, http.StatusNotFound, "", nil)
		return
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
	s.requests.WithLabelValues(epWellKnown, strconv.Itoa(http.StatusMovedPermanently)).Inc()
}

// reply writes body as XML, or only the status when body is nil.
func (s *server) reply(w http.ResponseWriter, endpoint string, status int, ctype string, body any) {
	s.requests.WithLabelValues(endpoint, strconv.Itoa(status)).Inc()
	if body == nil {
		http.Error(w, http.StatusText(status), status)
		return
	}
	out, err := xml.MarshalIndent(body, "", "  ")
	if err != nil {
		slog.Error("marshal response", "endpoint", endpoint, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ctype+"; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
	_, _ = w.Write([]byte("\n"))
}

// opsHandler serves health and metrics on the separate metrics port, which
// the ingress does not expose.
func opsHandler(g prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	}
	mux.HandleFunc("/healthz", ok)
	mux.HandleFunc("/readyz", ok)
	mux.Handle("/metrics", promhttp.HandlerFor(g, promhttp.HandlerOpts{}))
	return mux
}
