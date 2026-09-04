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
	ErrOwnerNotFound      = errors.New("owner not found")
	ErrOwnerTokenNotFound = errors.New("owner token not found")
)

type Owner struct {
	ID        string
	Provider  string
	Subject   string
	Email     string
	CreatedAt time.Time
}

type OwnerToken struct {
	TokenHash string
	OwnerID   string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (t OwnerToken) Expired(at time.Time) bool {
	return !at.Before(t.ExpiresAt)
}

type Identity struct {
	Provider string
	Subject  string
	Email    string
}
