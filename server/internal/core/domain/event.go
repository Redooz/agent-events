package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEventNotFound = errors.New("event not found")
	ErrNameRequired  = errors.New("event name is required")
)

type Event struct {
	ID          string
	UserID      string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return ErrNameRequired
	}

	return nil
}
