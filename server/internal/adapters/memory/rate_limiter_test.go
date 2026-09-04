package memory

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterAllowsUpToMaxWithinWindow(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(map[string]Limit{
		"event.create": {Max: 2, Window: time.Hour},
	})
	rl.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		allowed, err := rl.Allow(context.Background(), "owner:1", "event.create")
		if err != nil {
			t.Fatalf("Allow() #%d error = %v, want nil", i+1, err)
		}

		if !allowed {
			t.Fatalf("Allow() #%d = false, want true", i+1)
		}
	}

	allowed, err := rl.Allow(context.Background(), "owner:1", "event.create")
	if err != nil {
		t.Fatalf("Allow() error = %v, want nil", err)
	}

	if allowed {
		t.Error("Allow() beyond max = true, want false")
	}
}

func TestRateLimiterIsolatesKeysAndActions(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(map[string]Limit{
		"event.create":  {Max: 1, Window: time.Hour},
		"auth.exchange": {Max: 1, Window: time.Hour},
	})
	rl.now = func() time.Time { return now }

	if allowed, _ := rl.Allow(context.Background(), "owner:1", "event.create"); !allowed {
		t.Fatal("Allow() owner:1 event.create = false, want true")
	}

	if allowed, _ := rl.Allow(context.Background(), "owner:2", "event.create"); !allowed {
		t.Error("Allow() owner:2 event.create = false, want true (keys must be isolated)")
	}

	if allowed, _ := rl.Allow(context.Background(), "owner:1", "auth.exchange"); !allowed {
		t.Error("Allow() owner:1 auth.exchange = false, want true (actions must be isolated)")
	}

	if allowed, _ := rl.Allow(context.Background(), "owner:1", "event.create"); allowed {
		t.Error("Allow() repeat within window = true, want false")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(map[string]Limit{
		"event.create": {Max: 1, Window: time.Hour},
	})
	rl.now = func() time.Time { return now }

	if allowed, _ := rl.Allow(context.Background(), "owner:1", "event.create"); !allowed {
		t.Fatal("Allow() first = false, want true")
	}

	if allowed, _ := rl.Allow(context.Background(), "owner:1", "event.create"); allowed {
		t.Fatal("Allow() second within window = true, want false")
	}

	now = now.Add(time.Hour)
	if allowed, _ := rl.Allow(context.Background(), "owner:1", "event.create"); !allowed {
		t.Error("Allow() after window = false, want true")
	}
}

func TestRateLimiterUnlimitedAction(t *testing.T) {
	rl := NewRateLimiter(map[string]Limit{})

	allowed, err := rl.Allow(context.Background(), "owner:1", "anything")
	if err != nil {
		t.Fatalf("Allow() error = %v, want nil", err)
	}

	if !allowed {
		t.Error("Allow() unconfigured action = false, want true")
	}
}
