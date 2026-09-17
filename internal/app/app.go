package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"pulseroute/internal/api"
	"pulseroute/internal/config"
	"pulseroute/internal/database"
	"pulseroute/internal/delivery"
	"pulseroute/internal/redisx"
	"pulseroute/internal/security"
	"pulseroute/migrations"
	"syscall"
	"time"
)

func Run(mode string) error {
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, e := database.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		return e
	}
	defer db.Close()
	r, e := redisx.Open(cfg.RedisURL)
	if e != nil {
		return e
	}
	defer r.Close()
	if mode == "demo" {
		if e = migrations.Apply(ctx, db); e != nil {
			return e
		}
	}
	a := &api.Server{DB: db, Redis: r, Config: cfg, Log: log}
	handler := a.Handler()
	address := cfg.Address
	if mode == "worker" {
		address = cfg.WorkerAddress
		health := http.NewServeMux()
		health.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
		health.HandleFunc("GET /ready", func(w http.ResponseWriter, req *http.Request) {
			c, cancel := context.WithTimeout(req.Context(), 2*time.Second)
			defer cancel()
			if db.Ping(c) != nil || r.Ping(c).Err() != nil {
				w.WriteHeader(503)
				return
			}
			w.WriteHeader(200)
		})
		handler = health
	}
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan struct{})
	if mode == "worker" || mode == "demo" {
		worker := &delivery.Worker{Queue: delivery.Queue{DB: db, Lease: cfg.LeaseDuration}, Redis: r, Config: cfg, Log: log, Client: security.HTTPClient(cfg.HTTPTimeout, cfg.AllowPrivate)}
		go func() { defer close(done); worker.Run(ctx) }()
	} else {
		close(done)
	}
	errCh := make(chan error, 1)
	go func() { log.Info("server started", "mode", mode, "address", address); errCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case e = <-errCh:
		stop()
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		log.Error("HTTP shutdown", "error", err)
	}
	<-done
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
