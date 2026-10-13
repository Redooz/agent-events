package domain

import "time"

type Attendee struct {
	UserID    string
	Email     string
	CreatedAt time.Time
}
