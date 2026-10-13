package usecase

import "time"

type CreateEventInput struct {
	Name        string
	Description string
	Mode        string
	Date        time.Time
}

type UpdateEventInput struct {
	Name        string
	Description string
	Mode        string
}
