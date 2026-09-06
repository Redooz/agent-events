package domain

import (
	"errors"
	"slices"
	"time"
)

const (
	ProviderGoogle    = "google"
	ProviderApple     = "apple"
	ProviderMicrosoft = "microsoft"
)

var Providers = []string{ProviderGoogle, ProviderApple, ProviderMicrosoft}

func ValidProvider(provider string) bool {
	return slices.Contains(Providers, provider)
}

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserTokenNotFound = errors.New("user token not found")
)

type User struct {
	ID        string
	Provider  string
	Subject   string
	Email     string
	CreatedAt time.Time
}

type UserToken struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (t UserToken) Expired(at time.Time) bool {
	return !at.Before(t.ExpiresAt)
}

type Identity struct {
	Provider string
	Subject  string
	Email    string
}
