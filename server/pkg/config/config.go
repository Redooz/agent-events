package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Port     string
	Env      string
	LogLevel string

	DatabaseURL string

	GoogleClientID    string
	AppleClientID     string
	MicrosoftClientID string
	MicrosoftTenant   string

	OwnerTokenTTL     time.Duration
	MaxAgentsPerOwner int

	RateLimitEventsPerDay    int
	RateLimitExchangePerHour int
}

func Load() (Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("port", "8080")
	v.SetDefault("env", "development")
	v.SetDefault("log_level", "info")
	v.SetDefault("microsoft_tenant", "common")
	v.SetDefault("owner_token_ttl_hours", 720)
	v.SetDefault("max_agents_per_owner", 10)
	v.SetDefault("rate_limit_events_per_day", 20)
	v.SetDefault("rate_limit_exchange_per_hour", 30)

	cfg := Config{
		Port:              v.GetString("port"),
		Env:               strings.ToLower(v.GetString("env")),
		LogLevel:          strings.ToLower(v.GetString("log_level")),
		DatabaseURL:       v.GetString("database_url"),
		GoogleClientID:    v.GetString("google_client_id"),
		AppleClientID:     v.GetString("apple_client_id"),
		MicrosoftClientID: v.GetString("microsoft_client_id"),
		MicrosoftTenant:   v.GetString("microsoft_tenant"),
		OwnerTokenTTL:     time.Duration(v.GetInt("owner_token_ttl_hours")) * time.Hour,
		MaxAgentsPerOwner: v.GetInt("max_agents_per_owner"),

		RateLimitEventsPerDay:    v.GetInt("rate_limit_events_per_day"),
		RateLimitExchangePerHour: v.GetInt("rate_limit_exchange_per_hour"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("PORT must be a number between 1 and 65535, got %q", c.Port)
	}

	if c.IsProduction() {
		if c.DatabaseURL == "" {
			return fmt.Errorf("DATABASE_URL is required in production")
		}

		if c.GoogleClientID == "" && c.AppleClientID == "" && c.MicrosoftClientID == "" {
			return fmt.Errorf("at least one identity provider client id (GOOGLE_CLIENT_ID, APPLE_CLIENT_ID, MICROSOFT_CLIENT_ID) is required in production")
		}
	}

	if c.OwnerTokenTTL <= 0 {
		return fmt.Errorf("OWNER_TOKEN_TTL_HOURS must be positive, got %d", c.OwnerTokenTTL/time.Hour)
	}

	if c.MaxAgentsPerOwner < 1 {
		return fmt.Errorf("MAX_AGENTS_PER_OWNER must be at least 1, got %d", c.MaxAgentsPerOwner)
	}

	if c.RateLimitEventsPerDay < 1 {
		return fmt.Errorf("RATE_LIMIT_EVENTS_PER_DAY must be at least 1, got %d", c.RateLimitEventsPerDay)
	}

	if c.RateLimitExchangePerHour < 1 {
		return fmt.Errorf("RATE_LIMIT_EXCHANGE_PER_HOUR must be at least 1, got %d", c.RateLimitExchangePerHour)
	}

	return nil
}

func (c Config) IsProduction() bool {
	return c.Env == "production"
}
