package delivery

import (
	"math/rand/v2"
	"time"
)

func Retryable(status int) bool {
	return status == 0 || status == 408 || status == 429 || status >= 500
}
func Backoff(attempt, base int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 10 {
		attempt = 10
	}
	d := time.Duration(base) * time.Second * time.Duration(1<<uint(attempt-1))
	if d > time.Hour {
		d = time.Hour
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}
