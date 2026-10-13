package dto

import (
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase"
)

type CreateConnectionRequest struct {
	EventID  string `json:"event_id" validate:"required,uuid"`
	ToUserID string `json:"to_user_id" validate:"required,uuid"`
}

type ConnectionResponse struct {
	ID        string    `json:"id"`
	EventID   string    `json:"event_id"`
	EventName string    `json:"event_name,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	Method    string    `json:"method"`
	CreatedAt time.Time `json:"created_at"`
}

func NewConnectionResponse(view domain.ConnectionView) ConnectionResponse {
	return ConnectionResponse{
		ID:        view.ID,
		EventID:   view.EventID,
		EventName: view.EventName,
		UserID:    view.OtherUserID,
		Email:     view.OtherUserEmail,
		Method:    view.Method,
		CreatedAt: view.CreatedAt,
	}
}

func NewConnectionResponses(views []domain.ConnectionView) []ConnectionResponse {
	out := make([]ConnectionResponse, 0, len(views))
	for _, view := range views {
		out = append(out, NewConnectionResponse(view))
	}

	return out
}

func ConnectionResponseFrom(connection domain.Connection) ConnectionResponse {
	return ConnectionResponse{
		ID:        connection.ID,
		EventID:   connection.EventID,
		Method:    connection.Method,
		CreatedAt: connection.CreatedAt,
	}
}

type ConnectionRequestResponse struct {
	ID            string    `json:"id"`
	EventID       string    `json:"event_id"`
	EventName     string    `json:"event_name,omitempty"`
	FromUserID    string    `json:"from_user_id"`
	FromUserEmail string    `json:"from_user_email,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewConnectionRequestResponse(view domain.ConnectionRequestView) ConnectionRequestResponse {
	return ConnectionRequestResponse{
		ID:            view.ID,
		EventID:       view.EventID,
		EventName:     view.EventName,
		FromUserID:    view.FromUserID,
		FromUserEmail: view.FromUserEmail,
		Status:        view.Status,
		CreatedAt:     view.CreatedAt,
	}
}

func ConnectionRequestResponseFrom(request domain.ConnectionRequest) ConnectionRequestResponse {
	return ConnectionRequestResponse{
		ID:         request.ID,
		EventID:    request.EventID,
		FromUserID: request.FromUserID,
		Status:     request.Status,
		CreatedAt:  request.CreatedAt,
	}
}

func NewConnectionRequestResponses(views []domain.ConnectionRequestView) []ConnectionRequestResponse {
	out := make([]ConnectionRequestResponse, 0, len(views))
	for _, view := range views {
		out = append(out, NewConnectionRequestResponse(view))
	}

	return out
}

type RequestConnectionResultResponse struct {
	Status     string                    `json:"status"`
	Request    ConnectionRequestResponse `json:"request"`
	Connection *ConnectionResponse       `json:"connection,omitempty"`
}

func NewRequestConnectionResult(result usecase.RequestConnectionResult) RequestConnectionResultResponse {
	out := RequestConnectionResultResponse{
		Status:  domain.RequestStatusPending,
		Request: ConnectionRequestResponseFrom(result.Request),
	}

	if result.Request.Status == domain.RequestStatusAccepted {
		out.Status = usecase.ScanStatusConnected
	}

	if result.Connection != nil {
		connection := ConnectionResponseFrom(*result.Connection)
		out.Connection = &connection
	}

	return out
}
