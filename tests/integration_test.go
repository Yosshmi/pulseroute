package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"pulseroute/internal/api"
	"pulseroute/internal/config"
	"pulseroute/internal/database"
	"pulseroute/internal/delivery"
	"pulseroute/internal/receiver"
	"pulseroute/internal/redisx"
	"pulseroute/internal/security"
	"pulseroute/migrations"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegrationWorkflow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not configured")
	}
	if !strings.Contains(dbURL, "test") {
		t.Fatal("use a dedicated test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, e := database.Open(ctx, dbURL)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = migrations.Apply(ctx, db); e != nil {
		t.Fatal("migration not repeatable", e)
	}
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/15"
	}
	rd, e := redisx.Open(redisURL)
	if e != nil {
		t.Fatal(e)
	}
	defer rd.Close()
	if e = rd.Ping(ctx).Err(); e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{EncryptionKey: bytes.Repeat([]byte{9}, 32), Workers: 5, HTTPTimeout: 200 * time.Millisecond, LeaseDuration: 2 * time.Second, PollInterval: 10 * time.Millisecond, AllowPrivate: true}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := &api.Server{DB: db, Redis: rd, Config: cfg, Log: log}
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	request := func(method, path string, body any, key string) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		out := map[string]any{}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	check := func(want int, method, path string, body any, key string) map[string]any {
		t.Helper()
		status, out := request(method, path, body, key)
		if status != want {
			t.Fatalf("%s %s: got %d want %d: %v", method, path, status, want, out)
		}
		return out
	}
	account := security.Token("test_") + "@pulseroute.test"
	check(201, "POST", "/api/auth/register", map[string]string{"email": account, "password": "integration-password"}, "")
	project := check(201, "POST", "/api/projects", map[string]string{"name": "Integration"}, "")["id"].(string)
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.Exec(cleanup, "DELETE FROM delivery_attempts WHERE delivery_id IN(SELECT d.id FROM deliveries d JOIN events e ON e.id=d.event_id WHERE e.project_id=$1)", project)
		_, _ = db.Exec(cleanup, "DELETE FROM dead_letters WHERE delivery_id IN(SELECT d.id FROM deliveries d JOIN events e ON e.id=d.event_id WHERE e.project_id=$1)", project)
	})
	key := check(201, "POST", "/api/projects/"+project+"/api-keys", map[string]string{"name": "test"}, "")["key"].(string)
	secret := "integration-receiver-secret"
	destination := httptest.NewServer(receiver.New(secret))
	defer destination.Close()
	ep := check(201, "POST", "/api/projects/"+project+"/endpoints", map[string]any{"name": "recover", "url": destination.URL + "?failures=2", "secret": secret, "max_attempts": 3, "base_delay_seconds": 1, "rate_per_second": 1000}, "")["id"].(string)
	body := map[string]any{"event_id": "concurrent-1", "type": "order.created", "data": map[string]any{"value": 1499}}
	var wg sync.WaitGroup
	statuses := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s, _ := request("POST", "/v1/events", body, key); statuses <- s }()
	}
	wg.Wait()
	close(statuses)
	accepted := 0
	for s := range statuses {
		if s == 202 {
			accepted++
		} else if s != 200 {
			t.Fatalf("concurrent ingest: %d", s)
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d times", accepted)
	}
	body["data"] = map[string]int{"changed": 1}
	check(409, "POST", "/v1/events", body, key)
	var count int
	if e = db.QueryRow(ctx, "SELECT count(*) FROM deliveries d JOIN events e ON e.id=d.event_id WHERE e.project_id=$1", project).Scan(&count); e != nil || count != 1 {
		t.Fatalf("fanout not atomic: %d %v", count, e)
	}
	queue := delivery.Queue{DB: db, Lease: cfg.LeaseDuration}
	worker := &delivery.Worker{Queue: queue, Redis: rd, Config: cfg, Log: log, Client: security.HTTPClient(cfg.HTTPTimeout, true)}
	workCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(workCtx) }()
	defer func() { stop(); <-done }()
	await := func(eventID, status string) string {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			var id, got string
			e := db.QueryRow(ctx, "SELECT d.id,d.status FROM deliveries d JOIN events e ON e.id=d.event_id WHERE e.project_id=$1 AND e.event_id=$2", project, eventID).Scan(&id, &got)
			if e == nil && got == status {
				return id
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("event %s did not become %s", eventID, status)
		return ""
	}
	recovered := await("concurrent-1", "succeeded")
	detail := check(200, "GET", "/api/deliveries/"+recovered, nil, "")
	if len(detail["attempts"].([]any)) != 3 {
		t.Fatal("expected two failures then success")
	}
	check(200, "PATCH", "/api/endpoints/"+ep, map[string]any{"url": destination.URL + "?status=400"}, "")
	check(202, "POST", "/v1/events", map[string]any{"event_id": "permanent", "type": "invoice.failed", "data": map[string]int{"demo": 1}}, key)
	dead := await("permanent", "dead")
	detail = check(200, "GET", "/api/deliveries/"+dead, nil, "")
	if detail["attempt_count"] != float64(1) {
		t.Fatal("permanent failure retried")
	}
	check(200, "PATCH", "/api/endpoints/"+ep, map[string]any{"url": destination.URL}, "")
	check(202, "POST", "/api/dead-letters/"+dead+"/replay", nil, "")
	await("permanent", "succeeded")
	detail = check(200, "GET", "/api/deliveries/"+dead, nil, "")
	if len(detail["attempts"].([]any)) != 2 || detail["generation"] != float64(1) {
		t.Fatal("replay lost history")
	}
	check(200, "PATCH", "/api/endpoints/"+ep, map[string]any{"url": destination.URL + "?status=503", "max_attempts": 2}, "")
	check(202, "POST", "/v1/events", map[string]any{"event_id": "exhausted", "type": "payment.completed", "data": nil}, key)
	await("exhausted", "dead")
	check(200, "PATCH", "/api/endpoints/"+ep, map[string]any{"url": destination.URL + "?sleep=1s", "max_attempts": 1}, "")
	check(202, "POST", "/v1/events", map[string]any{"event_id": "timeout", "type": "payment.completed", "data": nil}, key)
	await("timeout", "dead")
	check(200, "GET", "/api/dashboard?project_id="+project, nil, "")
	check(201, "POST", "/api/auth/register", map[string]string{"email": security.Token("other_") + "@pulseroute.test", "password": "integration-password"}, "")
	check(404, "GET", "/api/projects/"+project, nil, "")
	check(404, "GET", "/api/deliveries/"+dead, nil, "")
	check(404, "POST", "/api/projects/"+project+"/api-keys", map[string]string{"name": "stolen"}, "")
	t.Run("redis_atomic_bucket", func(t *testing.T) {
		name := security.Token("bucket_")
		for i := 0; i < 3; i++ {
			ok, e := redisx.Allow(ctx, rd, name, 1, 3)
			if e != nil || !ok {
				t.Fatal(ok, e)
			}
		}
		ok, e := redisx.Allow(ctx, rd, name, 1, 3)
		if e != nil || ok {
			t.Fatal("bucket did not reject", e)
		}
	})
	t.Run("lease_fencing", func(t *testing.T) {
		stop()
		<-done
		check(200, "POST", "/api/auth/login", map[string]string{"email": account, "password": "integration-password"}, "")
		check(202, "POST", "/v1/events", map[string]any{"event_id": "lease-test", "type": "user.created", "data": nil}, key)
		j, e := queue.Claim(ctx)
		if e != nil {
			t.Fatal(e)
		}
		_, e = db.Exec(ctx, "UPDATE deliveries SET lease_until=now()-interval '1 second' WHERE id=$1", j.ID)
		if e != nil {
			t.Fatal(e)
		}
		j2, e := queue.Claim(ctx)
		if e != nil || j2.ID != j.ID {
			t.Fatal("reclaim", e)
		}
		if e = queue.Finish(ctx, j, 200, time.Millisecond, ""); e != delivery.ErrStale {
			t.Fatal("stale lease accepted", e)
		}
		if e = queue.Finish(ctx, j2, 200, time.Millisecond, ""); e != nil {
			t.Fatal(e)
		}
	})
	t.Log(fmt.Sprintf("project %s: concurrent idempotency, recovery, permanent failure, exhaustion, timeout, replay, authorization, Redis and lease fencing verified", project))
}
