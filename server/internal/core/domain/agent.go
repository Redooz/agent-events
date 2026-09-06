package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrAgentNotFound     = errors.New("agent not found")
	ErrAgentNameRequired = errors.New("agent name is required")
	ErrNotEventUser      = errors.New("actor does not own the event")
	ErrAgentKeyRevoked   = errors.New("agent key has been revoked")
)

type Agent struct {
	ID         string
	UserID     string
	Name       string
	KeyHash    string
	CreatedAt  time.Time
	RevokedAt  time.Time
	LastUsedAt time.Time
}

func (a Agent) Revoked() bool {
	return !a.RevokedAt.IsZero()
}

func (a Agent) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return ErrAgentNameRequired
	}

	return nil
}
