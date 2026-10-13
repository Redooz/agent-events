package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
)

const (
	EventModeOnline = "online"
	EventModeOnsite = "onsite"
)

var EventModes = []string{EventModeOnline, EventModeOnsite}

func ValidEventMode(mode string) bool {
	return slices.Contains(EventModes, mode)
}

var (
	ErrEventNotFound = errors.New("event not found")
	ErrNameRequired  = errors.New("event name is required")
	ErrInvalidMode   = errors.New("event mode must be online or onsite")
	ErrDateRequired  = errors.New("event date is required")
)

type Event struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Mode        string
	Date        time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e Event) Onsite() bool {
	return e.Mode == EventModeOnsite
}

func (e Event) Online() bool {
	return e.Mode == EventModeOnline
}

func (e Event) ModeEditable(at time.Time) bool {
	return at.Before(e.Date)
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return ErrNameRequired
	}

	if !ValidEventMode(e.Mode) {
		return ErrInvalidMode
	}

	if e.Date.IsZero() {
		return ErrDateRequired
	}

	return nil
}
