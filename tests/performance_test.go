package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"pulseroute/internal/api"
	"pulseroute/internal/config"
	"pulseroute/internal/database"
	"pulseroute/internal/delivery"
	"pulseroute/internal/democlient"
	"pulseroute/internal/receiver"
	"pulseroute/internal/redisx"
	"pulseroute/internal/security"
	"pulseroute/migrations"
	"testing"
	"time"
)

func TestPerformance(t *testing.T) {
	if os.Getenv("RUN_PERFORMANCE") != "1" {
		t.Skip("set RUN_PERFORMANCE=1 for measured worker comparison")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, e := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	rd, e := redisx.Open(os.Getenv("TEST_REDIS_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer rd.Close()
	secret := "performance-receiver-secret"
	destination := httptest.NewServer(receiver.New(secret))
	defer destination.Close()
	for _, size := range []int{1, 5, 10, 25} {
		t.Run(fmt.Sprintf("workers_%d", size), func(t *testing.T) {
			cfg := config.Config{EncryptionKey: bytes.Repeat([]byte{3}, 32), Workers: size, HTTPTimeout: time.Second, LeaseDuration: 10 * time.Second, PollInterval: 5 * time.Millisecond, AllowPrivate: true}
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			a := &api.Server{DB: db, Redis: rd, Config: cfg, Log: log}
			server := httptest.NewServer(a.Handler())
			defer server.Close()
			c := democlient.New(server.URL)
			_, e := c.Call("POST", "/api/auth/register", "", map[string]string{"email": security.Token("perf_") + "@pulseroute.test", "password": "performance-password"})
			if e != nil {
				t.Fatal(e)
			}
			p, e := c.Call("POST", "/api/projects", "", map[string]string{"name": "Measured worker comparison"})
			if e != nil {
				t.Fatal(e)
			}
			id := p["id"].(string)
			_, e = c.Call("POST", "/api/projects/"+id+"/endpoints", "", map[string]any{"name": "10ms receiver", "url": destination.URL + "?sleep=10ms", "secret": secret, "rate_per_second": 1000})
			if e != nil {
				t.Fatal(e)
			}
			k, e := c.Call("POST", "/api/projects/"+id+"/api-keys", "", map[string]string{"name": "performance"})
			if e != nil {
				t.Fatal(e)
			}
			key := k["key"].(string)
			const n = 60
			started := time.Now()
			errors := 0
			for i := 0; i < n; i++ {
				_, e = c.Call("POST", "/v1/events", key, map[string]any{"event_id": fmt.Sprintf("perf-%d", i), "type": "order.created", "data": map[string]bool{"synthetic": true}})
				if e != nil {
					errors++
				}
			}
			ingestion := time.Since(started).Seconds()
			workCtx, stop := context.WithCancel(ctx)
			w := &delivery.Worker{Queue: delivery.Queue{DB: db, Lease: cfg.LeaseDuration}, Redis: rd, Config: cfg, Log: log, Client: security.HTTPClient(cfg.HTTPTimeout, true)}
			done := make(chan struct{})
			started = time.Now()
			go func() { defer close(done); w.Run(workCtx) }()
			defer func() { stop(); <-done }()
			count := 0
			deadline := time.Now().Add(30 * time.Second)
			for count < n-errors && time.Now().Before(deadline) {
				if e = db.QueryRow(ctx, "SELECT count(*) FROM deliveries d JOIN events e ON e.id=d.event_id WHERE e.project_id=$1 AND d.status='succeeded'", id).Scan(&count); e != nil {
					t.Fatal(e)
				}
				time.Sleep(10 * time.Millisecond)
			}
			elapsed := time.Since(started).Seconds()
			if count != n-errors {
				t.Fatalf("only %d deliveries completed", count)
			}
			var avg, p95 float64
			if e = db.QueryRow(ctx, "SELECT avg(a.duration_ms),percentile_cont(0.95) WITHIN GROUP(ORDER BY a.duration_ms) FROM delivery_attempts a JOIN deliveries d ON d.id=a.delivery_id JOIN events e ON e.id=d.event_id WHERE e.project_id=$1", id).Scan(&avg, &p95); e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(map[string]any{"workers": size, "events": n, "ingestion_rps": float64(n) / ingestion, "delivery_rps": float64(count) / elapsed, "delivery_seconds": elapsed, "mean_attempt_ms": avg, "p95_attempt_ms": p95, "ingestion_errors": errors})
			t.Log(string(b))
			if errors > 0 {
				t.Fail()
			}
		})
	}
}
