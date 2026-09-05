package port

import "context"

type RateLimiter interface {
	Allow(ctx context.Context, key, action string) (bool, error)
}
