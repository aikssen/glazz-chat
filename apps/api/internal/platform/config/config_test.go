package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Environment != "development" || cfg.Runtime.APIAddress != ":8080" {
		t.Fatalf("unexpected runtime config: %+v", cfg.Runtime)
	}
	if cfg.Database.URL != developmentDatabaseURL {
		t.Fatalf("Database.URL = %q", cfg.Database.URL)
	}
	if cfg.Auth.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("AccessTokenTTL = %s", cfg.Auth.AccessTokenTTL)
	}
}

func TestLoadMapsProviderAliases(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("API_URL", "https://provider.example.test/v1/")
	t.Setenv("API_KEY", "development-only")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider.BaseURL != "https://provider.example.test/v1" {
		t.Fatalf("Provider.BaseURL = %q", cfg.Provider.BaseURL)
	}
	if cfg.Provider.APIKey != "development-only" {
		t.Fatal("Provider.APIKey was not loaded from compatibility alias")
	}
}

func TestLoadRejectsInvalidValuesWithoutLeakingSecrets(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "port", key: "API_PORT", value: "70000"},
		{name: "environment", key: "GLAZZ_ENV", value: "staging"},
		{name: "database", key: "DATABASE_URL", value: "secret-database-value"},
		{name: "redis", key: "REDIS_URL", value: "secret-redis-value"},
		{name: "cookie key", key: "COOKIE_SIGNING_KEY", value: "secret-cookie-value"},
		{name: "provider", key: "LLM_PROVIDER_BASE_URL", value: "secret-provider-value"},
		{name: "trusted proxy", key: "TRUSTED_PROXY_CIDRS", value: "secret-proxy-value"},
		{name: "maintenance", key: "MAINTENANCE_MODE", value: "secret-maintenance-value"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearEnvironment(t)
			t.Setenv(test.key, test.value)
			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil")
			}
			if strings.Contains(err.Error(), test.value) {
				t.Fatalf("error leaked configured value: %v", err)
			}
		})
	}
}

func TestLoadRejectsPartialOAuthConfiguration(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestProductionRequiresSecureInputs(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("GLAZZ_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func clearEnvironment(t *testing.T) {
	t.Helper()
	keys := []string{
		"GLAZZ_ENV", "API_PORT", "WEB_URL", "DATABASE_URL", "DATABASE_MAX_CONNECTIONS",
		"DATABASE_MIN_CONNECTIONS", "REDIS_URL", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"GOOGLE_CALLBACK_URL", "COOKIE_SIGNING_KEY", "COOKIE_SECURE", "TRUSTED_PROXY_CIDRS",
		"LLM_PROVIDER_BASE_URL", "LLM_PROVIDER_API_KEY", "API_URL", "API_KEY",
		"MAINTENANCE_MODE",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
