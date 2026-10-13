package domain

import (
	"errors"
	"time"
)

var ErrQRTokenNotFound = errors.New("qr token not found")

type QRToken struct {
	TokenHash string
	EventID   string
	UserID    string
	ExpiresAt time.Time
	UsedAt    time.Time
	CreatedAt time.Time
}

func (t QRToken) Expired(at time.Time) bool {
	return !at.Before(t.ExpiresAt)
}

func (t QRToken) Used() bool {
	return !t.UsedAt.IsZero()
}

type Scan struct {
	EventID   string
	ScannerID string
	ScannedID string
	CreatedAt time.Time
}
