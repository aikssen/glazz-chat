package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GLAZZ_ENV", "")
	t.Setenv("API_PORT", "")
	t.Setenv("LLM_PROVIDER_BASE_URL", "")
	t.Setenv("API_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.Port)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("API_PORT", "70000")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}

func TestLoadMapsProviderAliases(t *testing.T) {
	t.Setenv("LLM_PROVIDER_BASE_URL", "")
	t.Setenv("LLM_PROVIDER_API_KEY", "")
	t.Setenv("API_URL", "https://provider.example.test/v1/")
	t.Setenv("API_KEY", "development-only")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ProviderBaseURL != "https://provider.example.test/v1" {
		t.Fatalf("ProviderBaseURL = %q", cfg.ProviderBaseURL)
	}
	if cfg.ProviderAPIKey != "development-only" {
		t.Fatal("ProviderAPIKey was not loaded from compatibility alias")
	}
}

func TestLoadRejectsMalformedProviderURL(t *testing.T) {
	t.Setenv("LLM_PROVIDER_BASE_URL", "provider.example.test")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want malformed provider URL error")
	}
}
