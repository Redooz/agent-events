package domain

import (
	"errors"
	"time"
)

const (
	ConnectionMethodOnline = "online"
	ConnectionMethodQR     = "onsite_qr"
)

const (
	RequestStatusPending  = "pending"
	RequestStatusAccepted = "accepted"
	RequestStatusDeclined = "declined"
)

var (
	ErrConnectionNotFound = errors.New("connection not found")
	ErrRequestNotFound    = errors.New("connection request not found")
	ErrAlreadyConnected   = errors.New("users are already connected")
	ErrRequestExists      = errors.New("connection request already exists")
)

type Connection struct {
	ID        string
	EventID   string
	UserAID   string
	UserBID   string
	Method    string
	CreatedAt time.Time
}

type ConnectionView struct {
	Connection
	EventName      string
	OtherUserID    string
	OtherUserEmail string
}

type ConnectionRequest struct {
	ID          string
	EventID     string
	FromUserID  string
	ToUserID    string
	Status      string
	CreatedAt   time.Time
	RespondedAt time.Time
}

type ConnectionRequestView struct {
	ConnectionRequest
	EventName     string
	FromUserEmail string
}

func ConnectionPair(userA, userB string) (string, string) {
	if userA < userB {
		return userA, userB
	}

	return userB, userA
}
