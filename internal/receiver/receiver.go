package receiver

import (
	"io"
	"net/http"
	"pulseroute/internal/security"
	"strconv"
	"sync"
	"time"
)

type Receiver struct {
	Secret string
	mu     sync.Mutex
	counts map[string]int
}

func New(secret string) *Receiver { return &Receiver{Secret: secret, counts: make(map[string]int)} }
func (h *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		w.WriteHeader(200)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		w.WriteHeader(413)
		return
	}
	if !security.Verify(h.Secret, r.Header.Get("X-PulseRoute-Timestamp"), r.Header.Get("X-PulseRoute-Signature"), body, time.Now()) {
		w.WriteHeader(401)
		return
	}
	id := r.Header.Get("X-PulseRoute-Delivery-ID")
	h.mu.Lock()
	if len(h.counts) > 10000 {
		clear(h.counts)
	}
	h.counts[id]++
	count := h.counts[id]
	h.mu.Unlock()
	q := r.URL.Query()
	failures, _ := strconv.Atoi(q.Get("failures"))
	if count <= failures {
		w.WriteHeader(500)
		return
	}
	delay, _ := time.ParseDuration(q.Get("sleep"))
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
	}
	status, _ := strconv.Atoi(q.Get("status"))
	if status < 200 || status > 599 {
		status = 200
	}
	if status == 429 {
		w.Header().Set("Retry-After", "2")
	}
	w.WriteHeader(status)
}
