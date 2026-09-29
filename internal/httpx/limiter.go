package httpx

import (
	"context"
	"sync"
	"time"
)

// limiter is a small token bucket shared by all requests of one client.
type limiter struct {
	mu     sync.Mutex
	rate   float64 // tokens per second; 0 disables limiting
	burst  float64
	tokens float64
	last   time.Time
}

func newLimiter(rps float64, burst int) *limiter {
	if burst < 1 {
		burst = 1
	}
	return &limiter{rate: rps, burst: float64(burst), tokens: float64(burst), last: time.Now()}
}

func (l *limiter) wait(ctx context.Context) error {
	if l.rate <= 0 {
		return ctx.Err()
	}
	l.mu.Lock()
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * l.rate
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.last = now
	l.tokens--
	var d time.Duration
	if l.tokens < 0 {
		d = time.Duration(-l.tokens / l.rate * float64(time.Second))
	}
	l.mu.Unlock()
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
