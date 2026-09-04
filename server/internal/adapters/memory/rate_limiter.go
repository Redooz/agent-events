package memory

import (
	"context"
	"sync"
	"time"
)

type Limit struct {
	Max    int
	Window time.Duration
}

type bucketKey struct {
	action      string
	key         string
	windowStart int64
}

type RateLimiter struct {
	mu     sync.Mutex
	limits map[string]Limit
	counts map[bucketKey]int
	now    func() time.Time
}

func NewRateLimiter(limits map[string]Limit) *RateLimiter {
	copied := make(map[string]Limit, len(limits))
	for action, limit := range limits {
		copied[action] = limit
	}

	return &RateLimiter{
		limits: copied,
		counts: make(map[bucketKey]int),
		now:    time.Now,
	}
}

func (r *RateLimiter) Allow(_ context.Context, key, action string) (bool, error) {
	limit, ok := r.limits[action]
	if !ok {
		return true, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	start := now.Truncate(limit.Window).Unix()
	bucket := bucketKey{action: action, key: key, windowStart: start}

	if len(r.counts) > maxTrackedBuckets {
		r.prune(now)
	}

	r.counts[bucket]++

	return r.counts[bucket] <= limit.Max, nil
}

func (r *RateLimiter) prune(now time.Time) {
	for bucket := range r.counts {
		limit, ok := r.limits[bucket.action]
		if !ok {
			delete(r.counts, bucket)
			continue
		}

		if bucket.windowStart+int64(limit.Window/time.Second) <= now.Unix() {
			delete(r.counts, bucket)
		}
	}
}

const maxTrackedBuckets = 4096
