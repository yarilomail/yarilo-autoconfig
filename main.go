// Copyright (c) 2026 The yarilo-autoconfig Authors. All rights reserved.
// Use of this source code is governed by a GNU AGPLv3 style
// license that can be found in the LICENSE file.

// yarilo-autoconfig tells mail clients which hosts and ports to use for an
// address: Thunderbird autoconfig and Outlook POX Autodiscover.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	version = "dev"
	debug   bool
)

func debugf(msg string, args ...any) {
	if debug {
		slog.Debug(msg, args...)
	}
}

func main() {
	debug = strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug"
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		slog.Error("refusing to start", "err", err)
		os.Exit(1)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	srv := newServer(cfg, reg)

	public := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	ops := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           opsHandler(reg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 2)
	for _, hs := range []*http.Server{public, ops} {
		go func(hs *http.Server) {
			if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}(hs)
	}
	slog.Info("listening", "version", version, "addr", cfg.ListenAddr, "metrics_addr", cfg.MetricsAddr)

	select {
	case <-ctx.Done():
	case err := <-errc:
		slog.Error("listener failed", "err", err)
		os.Exit(1)
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = public.Shutdown(shutdown)
	_ = ops.Shutdown(shutdown)
}
