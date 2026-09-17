package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"io"
	"log/slog"
	"net/http"
	"pulseroute/internal/config"
	"pulseroute/internal/redisx"
	"pulseroute/internal/security"
	"strconv"
	"sync"
	"time"
)

type Worker struct {
	Queue  Queue
	Redis  *redis.Client
	Config config.Config
	Log    *slog.Logger
	Client *http.Client
}

func (w *Worker) Run(ctx context.Context) {
	jobs := make(chan Job, w.Config.Workers)
	slots := make(chan struct{}, w.Config.Workers)
	var wg sync.WaitGroup
	for i := 0; i < w.Config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				w.process(ctx, j)
				<-slots
			}
		}()
	}
	defer func() { close(jobs); wg.Wait(); w.Client.CloseIdleConnections() }()
	ticker := time.NewTicker(w.Config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case slots <- struct{}{}:
		}
		j, e := w.Queue.Claim(ctx)
		if e != nil {
			<-slots
			if !errors.Is(e, pgx.ErrNoRows) && ctx.Err() == nil {
				w.Log.Error("claim delivery", "error", e)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}
		select {
		case jobs <- j:
		case <-ctx.Done():
			<-slots
			return
		}
	}
}
func (w *Worker) process(ctx context.Context, j Job) {
	if ctx.Err() != nil {
		return
	}
	if !j.Active {
		w.deferJob(ctx, j, "cancelled", 0)
		return
	}
	allowed, e := redisx.Allow(ctx, w.Redis, "endpoint:"+j.EndpointID, j.Rate, j.Rate)
	if e != nil || !allowed {
		if e != nil {
			w.Log.Warn("endpoint limiter unavailable", "delivery_id", j.ID)
		}
		w.deferJob(ctx, j, "retrying", time.Second)
		return
	}
	secret, e := security.Decrypt(w.Config.EncryptionKey, j.Secret)
	if e != nil {
		w.Log.Error("decrypt endpoint secret", "endpoint_id", j.EndpointID)
		w.deferJob(ctx, j, "retrying", time.Minute)
		return
	}
	start := time.Now()
	status := 0
	reason := "network error"
	requestCtx, cancel := context.WithTimeout(ctx, w.Config.HTTPTimeout)
	defer cancel()
	req, e := http.NewRequestWithContext(requestCtx, http.MethodPost, j.URL, bytes.NewReader(j.Payload))
	if e == nil {
		stamp := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "PulseRoute/1.0")
		req.Header.Set("X-PulseRoute-Event-ID", j.EventID)
		req.Header.Set("X-PulseRoute-Delivery-ID", j.ID)
		req.Header.Set("X-PulseRoute-Timestamp", stamp)
		req.Header.Set("X-PulseRoute-Signature", security.Sign(secret, stamp, j.Payload))
		var response *http.Response
		response, e = w.Client.Do(req)
		if e == nil {
			status = response.StatusCode
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			response.Body.Close()
			reason = fmt.Sprintf("HTTP %d", status)
			if status >= 200 && status < 300 {
				reason = ""
			}
		}
	}
	if ctx.Err() != nil {
		return
	}
	if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
		reason = "request timeout"
	}
	duration := time.Since(start)
	finishCtx, finishCancel := context.WithTimeout(ctx, 5*time.Second)
	defer finishCancel()
	if e = w.Queue.Finish(finishCtx, j, status, duration, reason); e != nil {
		w.Log.Error("record delivery result", "delivery_id", j.ID, "error", e)
		return
	}
	w.Log.Info("delivery attempt", "delivery_id", j.ID, "event_id", j.EventID, "project_id", j.ProjectID, "endpoint_id", j.EndpointID, "attempt", j.Attempt, "status_code", status, "duration_ms", duration.Milliseconds())
}
func (w *Worker) deferJob(ctx context.Context, j Job, status string, delay time.Duration) {
	if e := w.Queue.Defer(ctx, j, status, delay); e != nil && ctx.Err() == nil {
		w.Log.Error("reschedule delivery", "delivery_id", j.ID, "error", e)
	}
}
