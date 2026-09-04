package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Port     string
	Env      string
	LogLevel string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("port", "8080")
	v.SetDefault("env", "development")
	v.SetDefault("log_level", "info")

	cfg := Config{
		Port:     v.GetString("port"),
		Env:      strings.ToLower(v.GetString("env")),
		LogLevel: strings.ToLower(v.GetString("log_level")),
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

	return nil
}

func (c Config) IsProduction() bool {
	return c.Env == "production"
}
