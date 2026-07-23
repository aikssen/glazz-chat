package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort            = 8080
	defaultShutdownTimeout = 10 * time.Second
)

type Config struct {
	Environment     string
	Port            int
	ShutdownTimeout time.Duration
	ProviderBaseURL string
	ProviderAPIKey  string
}

func Load() (Config, error) {
	cfg := Config{
		Environment:     valueOrDefault("GLAZZ_ENV", "development"),
		Port:            defaultPort,
		ShutdownTimeout: defaultShutdownTimeout,
		ProviderBaseURL: firstValue("LLM_PROVIDER_BASE_URL", "API_URL"),
		ProviderAPIKey:  firstValue("LLM_PROVIDER_API_KEY", "API_KEY"),
	}

	switch cfg.Environment {
	case "development", "test", "production":
	default:
		return Config{}, fmt.Errorf("parse GLAZZ_ENV: must be development, test, or production")
	}

	if rawPort := os.Getenv("API_PORT"); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("parse API_PORT: must be between 1 and 65535")
		}
		cfg.Port = port
	}

	if cfg.ProviderBaseURL != "" {
		parsed, err := url.ParseRequestURI(cfg.ProviderBaseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Config{}, fmt.Errorf("parse LLM_PROVIDER_BASE_URL (or API_URL alias): must be an absolute HTTP(S) URL")
		}
		cfg.ProviderBaseURL = strings.TrimRight(cfg.ProviderBaseURL, "/")
	}

	return cfg, nil
}

func (c Config) Address() string {
	return fmt.Sprintf(":%d", c.Port)
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func firstValue(primary, alias string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	return os.Getenv(alias)
}
