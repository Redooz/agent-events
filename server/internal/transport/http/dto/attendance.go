package dto

import "agent-events/server/internal/core/domain"

type AttendeeResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func NewAttendeeResponses(attendees []domain.Attendee) []AttendeeResponse {
	out := make([]AttendeeResponse, 0, len(attendees))
	for _, attendee := range attendees {
		out = append(out, AttendeeResponse{UserID: attendee.UserID, Email: attendee.Email})
	}

	return out
}
