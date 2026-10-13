package dto

import (
	"strings"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase"
)

type CreateEventRequest struct {
	Name        string    `json:"name" validate:"required,max=128"`
	Description string    `json:"description" validate:"max=1024"`
	Mode        string    `json:"mode" validate:"required,oneof=online onsite"`
	Date        time.Time `json:"date" validate:"required"`
}

type UpdateEventRequest struct {
	Name        string `json:"name" validate:"required,max=128"`
	Description string `json:"description" validate:"max=1024"`
	Mode        string `json:"mode" validate:"required,oneof=online onsite"`
}

type EventResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Mode        string    `json:"mode"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewEventResponse(event domain.Event) EventResponse {
	return EventResponse{
		ID:          event.ID,
		UserID:      event.UserID,
		Name:        event.Name,
		Description: event.Description,
		Mode:        event.Mode,
		Date:        event.Date,
		CreatedAt:   event.CreatedAt,
		UpdatedAt:   event.UpdatedAt,
	}
}

func NewEventResponses(events []domain.Event) []EventResponse {
	out := make([]EventResponse, 0, len(events))
	for _, event := range events {
		out = append(out, NewEventResponse(event))
	}

	return out
}

func (r CreateEventRequest) ToInput() usecase.CreateEventInput {
	return usecase.CreateEventInput{
		Name:        strings.TrimSpace(r.Name),
		Description: strings.TrimSpace(r.Description),
		Mode:        strings.TrimSpace(r.Mode),
		Date:        r.Date,
	}
}

func (r UpdateEventRequest) ToInput() usecase.UpdateEventInput {
	return usecase.UpdateEventInput{
		Name:        strings.TrimSpace(r.Name),
		Description: strings.TrimSpace(r.Description),
		Mode:        strings.TrimSpace(r.Mode),
	}
}
