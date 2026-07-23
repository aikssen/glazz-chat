package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	developmentDatabaseURL = "postgres://glazz:glazz@localhost:5432/glazz?sslmode=disable"
	developmentRedisURL    = "redis://localhost:6379/0"
)

type Config struct {
	Runtime   Runtime
	Database  Database
	Redis     Redis
	OAuth     OAuth
	Auth      Auth
	Cookies   Cookies
	Provider  Provider
	Admin     Admin
	Telemetry Telemetry
}

type Runtime struct {
	Environment     string
	APIAddress      string
	WebURL          string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
	MaxBodyBytes    int64
	Maintenance     bool
	TrustedProxies  []netip.Prefix
	AllowedOrigins  []string
}

type Database struct {
	URL              string
	MaxConnections   int32
	MinConnections   int32
	MaxLifetime      time.Duration
	MaxIdleTime      time.Duration
	HealthTimeout    time.Duration
	MigrateOnStartup bool
}

type Redis struct {
	URL           string
	Prefix        string
	HealthTimeout time.Duration
}

type OAuth struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

type Auth struct {
	Issuer          string
	Audience        string
	ActiveKeyID     string
	PrivateKeyPath  string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	RecentAuthTTL   time.Duration
	TermsVersion    string
	PrivacyVersion  string
}

type Cookies struct {
	SigningKey []byte
	Domain     string
	Secure     bool
	SameSite   string
}

type Provider struct {
	Kind         string
	BaseURL      string
	APIKey       string
	DefaultModel string
}

type Admin struct {
	BootstrapEmails map[string]struct{}
}

type Telemetry struct {
	ServiceName  string
	OTLPEndpoint string
	MetricsPath  string
}

func Load() (Config, error) {
	environment := valueOrDefault("GLAZZ_ENV", "development")
	if environment != "development" && environment != "test" && environment != "production" {
		return Config{}, errors.New("parse GLAZZ_ENV: must be development, test, or production")
	}

	port, err := integer("API_PORT", 8080, 1, 65535)
	if err != nil {
		return Config{}, err
	}
	maxBodyBytes, err := integer64("HTTP_MAX_BODY_BYTES", 1<<20, 1024, 16<<20)
	if err != nil {
		return Config{}, err
	}
	maxConnections, err := integer("DATABASE_MAX_CONNECTIONS", 20, 1, 200)
	if err != nil {
		return Config{}, err
	}
	minConnections, err := integer("DATABASE_MIN_CONNECTIONS", 2, 0, maxConnections)
	if err != nil {
		return Config{}, err
	}

	webURL := valueOrDefault("WEB_URL", "http://localhost:3000")
	if err := absoluteHTTPURL("WEB_URL", webURL); err != nil {
		return Config{}, err
	}
	databaseURL := valueOrDefault("DATABASE_URL", developmentDatabaseURL)
	if err := connectionURL("DATABASE_URL", databaseURL, "postgres", "postgresql"); err != nil {
		return Config{}, err
	}
	redisURL := valueOrDefault("REDIS_URL", developmentRedisURL)
	if err := connectionURL("REDIS_URL", redisURL, "redis", "rediss"); err != nil {
		return Config{}, err
	}

	oauthClientID := os.Getenv("GOOGLE_CLIENT_ID")
	oauthClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if (oauthClientID == "") != (oauthClientSecret == "") {
		return Config{}, errors.New("configure Google OAuth: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set together")
	}
	oauthEnabled := oauthClientID != ""
	callbackURL := valueOrDefault("GOOGLE_CALLBACK_URL", "http://localhost:8080/api/v1/auth/google/callback")
	if oauthEnabled {
		if err := absoluteHTTPURL("GOOGLE_CALLBACK_URL", callbackURL); err != nil {
			return Config{}, err
		}
	}

	cookieKey, err := cookieSigningKey(environment)
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := boolean("COOKIE_SECURE", environment == "production")
	if err != nil {
		return Config{}, err
	}
	migrateOnStartup, err := boolean("DATABASE_MIGRATE_ON_STARTUP", environment != "production")
	if err != nil {
		return Config{}, err
	}
	maintenance, err := boolean("MAINTENANCE_MODE", false)
	if err != nil {
		return Config{}, err
	}
	trustedProxies, err := prefixes(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return Config{}, err
	}

	providerBaseURL := firstValue("LLM_PROVIDER_BASE_URL", "API_URL")
	if providerBaseURL != "" {
		if err := absoluteHTTPURL("LLM_PROVIDER_BASE_URL (or API_URL alias)", providerBaseURL); err != nil {
			return Config{}, err
		}
		providerBaseURL = strings.TrimRight(providerBaseURL, "/")
	}

	cfg := Config{
		Runtime: Runtime{
			Environment:     environment,
			APIAddress:      fmt.Sprintf(":%d", port),
			WebURL:          strings.TrimRight(webURL, "/"),
			ShutdownTimeout: duration("SHUTDOWN_TIMEOUT", 10*time.Second),
			RequestTimeout:  duration("HTTP_REQUEST_TIMEOUT", 30*time.Second),
			MaxBodyBytes:    maxBodyBytes,
			Maintenance:     maintenance,
			TrustedProxies:  trustedProxies,
			AllowedOrigins:  stringList(valueOrDefault("CORS_ALLOWED_ORIGINS", webURL)),
		},
		Database: Database{
			URL:              databaseURL,
			MaxConnections:   int32(maxConnections),
			MinConnections:   int32(minConnections),
			MaxLifetime:      duration("DATABASE_MAX_LIFETIME", time.Hour),
			MaxIdleTime:      duration("DATABASE_MAX_IDLE_TIME", 30*time.Minute),
			HealthTimeout:    duration("DATABASE_HEALTH_TIMEOUT", 2*time.Second),
			MigrateOnStartup: migrateOnStartup,
		},
		Redis: Redis{
			URL:           redisURL,
			Prefix:        valueOrDefault("REDIS_PREFIX", "glazz"),
			HealthTimeout: duration("REDIS_HEALTH_TIMEOUT", 2*time.Second),
		},
		OAuth: OAuth{
			Enabled:      oauthEnabled,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			CallbackURL:  callbackURL,
		},
		Auth: Auth{
			Issuer:          valueOrDefault("JWT_ISSUER", "http://localhost:8080"),
			Audience:        valueOrDefault("JWT_AUDIENCE", "glazz-web"),
			ActiveKeyID:     valueOrDefault("JWT_ACTIVE_KID", "local-1"),
			PrivateKeyPath:  os.Getenv("JWT_PRIVATE_KEY_PATH"),
			AccessTokenTTL:  duration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: duration("AUTH_REFRESH_TTL", 30*24*time.Hour),
			RecentAuthTTL:   duration("AUTH_RECENT_TTL", 15*time.Minute),
			TermsVersion:    valueOrDefault("TERMS_VERSION", "2026-07-23"),
			PrivacyVersion:  valueOrDefault("PRIVACY_VERSION", "2026-07-23"),
		},
		Cookies: Cookies{
			SigningKey: cookieKey,
			Domain:     os.Getenv("COOKIE_DOMAIN"),
			Secure:     cookieSecure,
			SameSite:   valueOrDefault("COOKIE_SAME_SITE", "lax"),
		},
		Provider: Provider{
			Kind:         valueOrDefault("LLM_PROVIDER_KIND", "fake"),
			BaseURL:      providerBaseURL,
			APIKey:       firstValue("LLM_PROVIDER_API_KEY", "API_KEY"),
			DefaultModel: valueOrDefault("LLM_DEFAULT_MODEL", "deepseek-v4-flash"),
		},
		Admin: Admin{BootstrapEmails: normalizedSet(os.Getenv("BOOTSTRAP_ADMIN_EMAILS"))},
		Telemetry: Telemetry{
			ServiceName:  valueOrDefault("OTEL_SERVICE_NAME", "glazz-api"),
			OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
			MetricsPath:  valueOrDefault("METRICS_PATH", "/metrics"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Runtime.ShutdownTimeout <= 0 {
		return errors.New("validate SHUTDOWN_TIMEOUT: must be positive")
	}
	if c.Runtime.RequestTimeout <= 0 {
		return errors.New("validate HTTP_REQUEST_TIMEOUT: must be positive")
	}
	if c.Database.MaxLifetime <= 0 {
		return errors.New("validate DATABASE_MAX_LIFETIME: must be positive")
	}
	if c.Database.MaxIdleTime <= 0 {
		return errors.New("validate DATABASE_MAX_IDLE_TIME: must be positive")
	}
	if c.Database.HealthTimeout <= 0 {
		return errors.New("validate DATABASE_HEALTH_TIMEOUT: must be positive")
	}
	if c.Redis.HealthTimeout <= 0 {
		return errors.New("validate REDIS_HEALTH_TIMEOUT: must be positive")
	}
	if c.Auth.AccessTokenTTL <= 0 || c.Auth.AccessTokenTTL > 15*time.Minute {
		return errors.New("validate JWT_ACCESS_TTL: must be between 1ns and 15m")
	}
	if c.Auth.RefreshTokenTTL <= c.Auth.AccessTokenTTL {
		return errors.New("validate AUTH_REFRESH_TTL: must exceed JWT_ACCESS_TTL")
	}
	if c.Auth.RecentAuthTTL <= 0 {
		return errors.New("validate AUTH_RECENT_TTL: must be positive")
	}
	if c.Runtime.Environment == "production" {
		if !c.OAuth.Enabled {
			return errors.New("validate production config: Google OAuth credentials are required")
		}
		if c.Auth.PrivateKeyPath == "" {
			return errors.New("validate production config: JWT_PRIVATE_KEY_PATH is required")
		}
		if len(c.Cookies.SigningKey) < 32 {
			return errors.New("validate production config: COOKIE_SIGNING_KEY must decode to at least 32 bytes")
		}
		if !c.Cookies.Secure {
			return errors.New("validate production config: COOKIE_SECURE must be true")
		}
	}
	if c.Cookies.SameSite != "lax" && c.Cookies.SameSite != "strict" {
		return errors.New("validate COOKIE_SAME_SITE: must be lax or strict")
	}
	if strings.TrimSpace(c.Redis.Prefix) == "" || strings.ContainsAny(c.Redis.Prefix, " \t\r\n:") {
		return errors.New("validate REDIS_PREFIX: must be non-empty and contain no spaces or colon")
	}
	return nil
}

func cookieSigningKey(environment string) ([]byte, error) {
	raw := os.Getenv("COOKIE_SIGNING_KEY")
	if raw == "" {
		if environment == "production" {
			return nil, errors.New("parse COOKIE_SIGNING_KEY: required in production")
		}
		return []byte("local-development-cookie-key-32b"), nil
	}
	key, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(key) < 32 {
		return nil, errors.New("parse COOKIE_SIGNING_KEY: must be base64url without padding and decode to at least 32 bytes")
	}
	return key, nil
}

func duration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return -1
	}
	return parsed
}

func integer(key string, fallback, minimum, maximum int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("parse %s: must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func integer64(key string, fallback, minimum, maximum int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("parse %s: must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func boolean(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("parse %s: must be true or false", key)
	}
	return value, nil
}

func absoluteHTTPURL(key, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("parse %s: must be an absolute HTTP(S) URL", key)
	}
	return nil
}

func connectionURL(key, value string, schemes ...string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("parse %s: must be an absolute connection URL", key)
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}
	return fmt.Errorf("parse %s: unsupported URL scheme", key)
}

func prefixes(raw string) ([]netip.Prefix, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	values := stringList(raw)
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, errors.New("parse TRUSTED_PROXY_CIDRS: entries must be valid CIDR prefixes")
		}
		result = append(result, prefix)
	}
	return result, nil
}

func stringList(raw string) []string {
	var result []string
	for _, value := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func normalizedSet(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, value := range stringList(raw) {
		result[strings.ToLower(value)] = struct{}{}
	}
	return result
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
