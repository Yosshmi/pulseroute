package delivery

import (
	"testing"
	"time"
)

func TestRetryPolicy(t *testing.T) {
	for _, n := range []int{0, 408, 429, 500, 503} {
		if !Retryable(n) {
			t.Errorf("must retry %d", n)
		}
	}
	for _, n := range []int{200, 301, 400, 401, 404, 422} {
		if Retryable(n) {
			t.Errorf("must not retry %d", n)
		}
	}
	for attempt := 1; attempt <= 10; attempt++ {
		max := time.Duration(5*(1<<uint(attempt-1))) * time.Second
		if max > time.Hour {
			max = time.Hour
		}
		for i := 0; i < 100; i++ {
			d := Backoff(attempt, 5)
			if d < max/2 || d > max {
				t.Fatalf("invalid backoff %s", d)
			}
		}
	}
}
