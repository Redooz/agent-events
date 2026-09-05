package dto

import (
	"strings"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase"
)

type ExchangeRequest struct {
	Provider string `json:"provider" validate:"required,oneof=google apple microsoft"`
	IDToken  string `json:"id_token" validate:"required"`
}

type OwnerResponse struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type ExchangeResponse struct {
	Token     string        `json:"token"`
	ExpiresAt time.Time     `json:"expires_at"`
	Owner     OwnerResponse `json:"owner"`
}

func NewOwnerResponse(owner domain.Owner) OwnerResponse {
	return OwnerResponse{
		ID:        owner.ID,
		Provider:  owner.Provider,
		Email:     owner.Email,
		CreatedAt: owner.CreatedAt,
	}
}

func (r ExchangeRequest) ToInput(clientIP string) usecase.ExchangeInput {
	return usecase.ExchangeInput{
		Provider: strings.TrimSpace(r.Provider),
		IDToken:  strings.TrimSpace(r.IDToken),
		ClientIP: clientIP,
	}
}
