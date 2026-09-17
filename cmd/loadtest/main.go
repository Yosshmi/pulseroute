package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"pulseroute/internal/democlient"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	base := flag.String("url", "http://localhost:8080", "API URL")
	n := flag.Int("events", 100, "requests")
	concurrency := flag.Int("concurrency", 5, "concurrent clients")
	flag.Parse()
	key := os.Getenv("API_KEY")
	if key == "" || *n < 1 || *concurrency < 1 || *concurrency > 128 {
		fmt.Fprintln(os.Stderr, "API_KEY and positive events/concurrency <=128 required")
		os.Exit(1)
	}
	c := democlient.New(*base)
	jobs := make(chan int)
	var wg sync.WaitGroup
	var failures atomic.Int64
	var mu sync.Mutex
	latencies := make([]float64, 0, *n)
	started := time.Now()
	run := started.UnixNano()
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				start := time.Now()
				_, e := c.Call("POST", "/v1/events", key, map[string]any{"event_id": fmt.Sprintf("load-%d-%d", run, j), "type": "order.created", "data": map[string]any{"synthetic": true, "sequence": j}})
				if e != nil {
					failures.Add(1)
				}
				mu.Lock()
				latencies = append(latencies, time.Since(start).Seconds()*1000)
				mu.Unlock()
			}
		}()
	}
	for j := 0; j < *n; j++ {
		jobs <- j
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(started).Seconds()
	sort.Float64s(latencies)
	result := map[string]any{"requests": *n, "errors": failures.Load(), "error_rate": float64(failures.Load()) / float64(*n), "elapsed_seconds": elapsed, "request_throughput_per_second": float64(*n) / elapsed, "p50_ms": latencies[len(latencies)/2], "p95_ms": latencies[int(float64(len(latencies)-1)*.95)]}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if failures.Load() > 0 {
		os.Exit(1)
	}
}
