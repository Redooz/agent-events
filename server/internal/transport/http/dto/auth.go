package dto

import (
	"strings"
	"time"

	"agent-events/server/internal/core/domain"
	"agent-events/server/internal/core/usecase/types"
)

type ExchangeRequest struct {
	Provider string `json:"provider" validate:"required,oneof=google apple microsoft"`
	IDToken  string `json:"id_token" validate:"required"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type ExchangeResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

func NewUserResponse(user domain.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Provider:  user.Provider,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

func (r ExchangeRequest) ToInput(clientIP string) types.ExchangeInput {
	return types.ExchangeInput{
		Provider: strings.TrimSpace(r.Provider),
		IDToken:  strings.TrimSpace(r.IDToken),
		ClientIP: clientIP,
	}
}
