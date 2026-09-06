package config

import (
	"fmt"
	"net"
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

	UserTokenTTL     time.Duration
	MaxAgentsPerUser int

	RateLimitEventsPerDay    int
	RateLimitExchangePerHour int

	TrustedProxies []*net.IPNet
}

func Load() (Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("port", "8080")
	v.SetDefault("env", "development")
	v.SetDefault("log_level", "info")
	v.SetDefault("microsoft_tenant", "common")
	v.SetDefault("user_token_ttl_hours", 720)
	v.SetDefault("max_agents_per_user", 10)
	v.SetDefault("rate_limit_events_per_day", 20)
	v.SetDefault("rate_limit_exchange_per_hour", 30)

	trustedProxies, err := parseTrustedProxies(v.GetString("trusted_proxies"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:              v.GetString("port"),
		Env:               strings.ToLower(v.GetString("env")),
		LogLevel:          strings.ToLower(v.GetString("log_level")),
		DatabaseURL:       v.GetString("database_url"),
		GoogleClientID:    v.GetString("google_client_id"),
		AppleClientID:     v.GetString("apple_client_id"),
		MicrosoftClientID: v.GetString("microsoft_client_id"),
		MicrosoftTenant:   v.GetString("microsoft_tenant"),
		UserTokenTTL:      time.Duration(v.GetInt("user_token_ttl_hours")) * time.Hour,
		MaxAgentsPerUser:  v.GetInt("max_agents_per_user"),

		RateLimitEventsPerDay:    v.GetInt("rate_limit_events_per_day"),
		RateLimitExchangePerHour: v.GetInt("rate_limit_exchange_per_hour"),

		TrustedProxies: trustedProxies,
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

	if !c.IsDevelopment() {
		if c.GoogleClientID == "" && c.AppleClientID == "" && c.MicrosoftClientID == "" {
			return fmt.Errorf("at least one identity provider client id (GOOGLE_CLIENT_ID, APPLE_CLIENT_ID, MICROSOFT_CLIENT_ID) is required outside development")
		}
	}

	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.UserTokenTTL <= 0 {
		return fmt.Errorf("USER_TOKEN_TTL_HOURS must be positive, got %d", c.UserTokenTTL/time.Hour)
	}

	if c.MaxAgentsPerUser < 1 {
		return fmt.Errorf("MAX_AGENTS_PER_USER must be at least 1, got %d", c.MaxAgentsPerUser)
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

func (c Config) IsDevelopment() bool {
	return c.Env == "development"
}

func parseTrustedProxies(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	networks := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if !strings.Contains(part, "/") {
			ip := net.ParseIP(part)
			if ip == nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES entry %q is not a valid IP or CIDR range", part)
			}

			bits := 32
			if ip.To4() == nil {
				bits = 128
			}

			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})

			continue
		}

		_, network, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES entry %q is not a valid IP or CIDR range", part)
		}

		networks = append(networks, network)
	}

	return networks, nil
}
