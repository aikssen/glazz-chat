package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestLoadExplicitEnvironment(t *testing.T) {
	setValidEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Runtime.Environment != "development" || cfg.Runtime.APIAddress != ":8080" {
		t.Fatalf("unexpected runtime config: %+v", cfg.Runtime)
	}
	if cfg.Database.URL != "postgres://glazz:glazz@localhost:5432/glazz?sslmode=disable" {
		t.Fatal("Database.URL was not loaded from the environment")
	}
	if cfg.Auth.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("AccessTokenTTL = %s", cfg.Auth.AccessTokenTTL)
	}
}

func TestLoadRequiresDeclaredEnvironment(t *testing.T) {
	clearEnvironment(t)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "GLAZZ_ENV") {
		t.Fatalf("Load() error = %v, want missing GLAZZ_ENV", err)
	}
}

func TestLoadMapsProviderAliases(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("LLM_PROVIDER_BASE_URL", "")
	t.Setenv("LLM_PROVIDER_API_KEY", "")
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
			setValidEnvironment(t)
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
	setValidEnvironment(t)
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestProductionRequiresSecureInputs(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("GLAZZ_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	clearEnvironment(t)
	values := map[string]string{
		"GLAZZ_ENV":                   "development",
		"API_PORT":                    "8080",
		"WEB_URL":                     "http://localhost:3000",
		"DATABASE_URL":                "postgres://glazz:glazz@localhost:5432/glazz?sslmode=disable",
		"DATABASE_MAX_CONNECTIONS":    "20",
		"DATABASE_MIN_CONNECTIONS":    "2",
		"DATABASE_MAX_LIFETIME":       "1h",
		"DATABASE_MAX_IDLE_TIME":      "30m",
		"DATABASE_HEALTH_TIMEOUT":     "2s",
		"DATABASE_MIGRATE_ON_STARTUP": "true",
		"REDIS_URL":                   "redis://localhost:6379/0",
		"REDIS_PREFIX":                "glazz-test",
		"REDIS_HEALTH_TIMEOUT":        "2s",
		"SHUTDOWN_TIMEOUT":            "10s",
		"HTTP_REQUEST_TIMEOUT":        "30s",
		"HTTP_MAX_BODY_BYTES":         "1048576",
		"MAINTENANCE_MODE":            "false",
		"CORS_ALLOWED_ORIGINS":        "http://localhost:3000",
		"TRUSTED_PROXY_CIDRS":         "",
		"GOOGLE_CLIENT_ID":            "",
		"GOOGLE_CLIENT_SECRET":        "",
		"GOOGLE_CALLBACK_URL":         "http://localhost:8080/api/v1/auth/google/callback",
		"JWT_ISSUER":                  "http://localhost:8080",
		"JWT_AUDIENCE":                "glazz-web",
		"JWT_ACTIVE_KID":              "test-1",
		"JWT_PRIVATE_KEY_PATH":        "",
		"JWT_ACCESS_TTL":              "15m",
		"AUTH_REFRESH_TTL":            "720h",
		"AUTH_RECENT_TTL":             "15m",
		"COOKIE_SIGNING_KEY":          base64.RawURLEncoding.EncodeToString([]byte("01234567890123456789012345678901")),
		"COOKIE_DOMAIN":               "",
		"COOKIE_SECURE":               "false",
		"COOKIE_SAME_SITE":            "lax",
		"TERMS_VERSION":               "terms-test",
		"PRIVACY_VERSION":             "privacy-test",
		"BOOTSTRAP_ADMIN_EMAILS":      "",
		"OTEL_SERVICE_NAME":           "glazz-test",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "",
		"METRICS_PATH":                "/metrics",
		"LLM_PROVIDER_KIND":           "fake",
		"LLM_PROVIDER_BASE_URL":       "",
		"LLM_PROVIDER_API_KEY":        "",
		"LLM_DEFAULT_MODEL":           "deepseek-v4-flash",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}

func clearEnvironment(t *testing.T) {
	t.Helper()
	keys := []string{
		"GLAZZ_ENV_FILE", "GLAZZ_ENV", "API_PORT", "WEB_URL", "DATABASE_URL",
		"DATABASE_MAX_CONNECTIONS", "DATABASE_MIN_CONNECTIONS", "DATABASE_MAX_LIFETIME",
		"DATABASE_MAX_IDLE_TIME", "DATABASE_HEALTH_TIMEOUT", "DATABASE_MIGRATE_ON_STARTUP",
		"REDIS_URL", "REDIS_PREFIX", "REDIS_HEALTH_TIMEOUT", "SHUTDOWN_TIMEOUT",
		"HTTP_REQUEST_TIMEOUT", "HTTP_MAX_BODY_BYTES", "MAINTENANCE_MODE",
		"CORS_ALLOWED_ORIGINS", "TRUSTED_PROXY_CIDRS", "GOOGLE_CLIENT_ID",
		"GOOGLE_CLIENT_SECRET", "GOOGLE_CALLBACK_URL", "JWT_ISSUER", "JWT_AUDIENCE",
		"JWT_ACTIVE_KID", "JWT_PRIVATE_KEY_PATH", "JWT_ACCESS_TTL", "AUTH_REFRESH_TTL",
		"AUTH_RECENT_TTL", "COOKIE_SIGNING_KEY", "COOKIE_DOMAIN", "COOKIE_SECURE",
		"COOKIE_SAME_SITE", "TERMS_VERSION", "PRIVACY_VERSION", "BOOTSTRAP_ADMIN_EMAILS",
		"OTEL_SERVICE_NAME", "OTEL_EXPORTER_OTLP_ENDPOINT", "METRICS_PATH",
		"LLM_PROVIDER_KIND", "LLM_PROVIDER_BASE_URL", "LLM_PROVIDER_API_KEY",
		"LLM_DEFAULT_MODEL", "API_URL", "API_KEY",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
