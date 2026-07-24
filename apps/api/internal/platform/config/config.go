package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
	TestMode     bool
	ClientID     string
	ClientSecret string
	CallbackURL  string
	TestEmail    string
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
	if err := loadEnvironmentFile(); err != nil {
		return Config{}, err
	}
	environment, err := nonEmptyValue("GLAZZ_ENV")
	if err != nil {
		return Config{}, err
	}
	if environment != "development" && environment != "test" && environment != "production" {
		return Config{}, errors.New("parse GLAZZ_ENV: must be development, test, or production")
	}

	port, err := integer("API_PORT", 1, 65535)
	if err != nil {
		return Config{}, err
	}
	maxBodyBytes, err := integer64("HTTP_MAX_BODY_BYTES", 1024, 16<<20)
	if err != nil {
		return Config{}, err
	}
	maxConnections, err := integer("DATABASE_MAX_CONNECTIONS", 1, 200)
	if err != nil {
		return Config{}, err
	}
	minConnections, err := integer("DATABASE_MIN_CONNECTIONS", 0, maxConnections)
	if err != nil {
		return Config{}, err
	}

	webURL, err := nonEmptyValue("WEB_URL")
	if err != nil {
		return Config{}, err
	}
	if err := absoluteHTTPURL("WEB_URL", webURL); err != nil {
		return Config{}, err
	}
	databaseURL, err := nonEmptyValue("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	if err := connectionURL("DATABASE_URL", databaseURL, "postgres", "postgresql"); err != nil {
		return Config{}, err
	}
	redisURL, err := nonEmptyValue("REDIS_URL")
	if err != nil {
		return Config{}, err
	}
	if err := connectionURL("REDIS_URL", redisURL, "redis", "rediss"); err != nil {
		return Config{}, err
	}

	oauthClientID, err := value("GOOGLE_CLIENT_ID")
	if err != nil {
		return Config{}, err
	}
	oauthClientSecret, err := value("GOOGLE_CLIENT_SECRET")
	if err != nil {
		return Config{}, err
	}
	if (oauthClientID == "") != (oauthClientSecret == "") {
		return Config{}, errors.New("configure Google OAuth: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set together")
	}
	oauthTestMode, err := boolean("OAUTH_TEST_MODE")
	if err != nil {
		return Config{}, err
	}
	oauthTestEmail, err := value("OAUTH_TEST_EMAIL")
	if err != nil {
		return Config{}, err
	}
	oauthEnabled := oauthClientID != "" || oauthTestMode
	callbackURL, err := nonEmptyValue("GOOGLE_CALLBACK_URL")
	if err != nil {
		return Config{}, err
	}
	if err := absoluteHTTPURL("GOOGLE_CALLBACK_URL", callbackURL); err != nil {
		return Config{}, err
	}

	cookieKey, err := cookieSigningKey()
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := boolean("COOKIE_SECURE")
	if err != nil {
		return Config{}, err
	}
	migrateOnStartup, err := boolean("DATABASE_MIGRATE_ON_STARTUP")
	if err != nil {
		return Config{}, err
	}
	maintenance, err := boolean("MAINTENANCE_MODE")
	if err != nil {
		return Config{}, err
	}
	trustedProxyValue, err := value("TRUSTED_PROXY_CIDRS")
	if err != nil {
		return Config{}, err
	}
	trustedProxies, err := prefixes(trustedProxyValue)
	if err != nil {
		return Config{}, err
	}
	allowedOriginValue, err := nonEmptyValue("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return Config{}, err
	}
	allowedOrigins := stringList(allowedOriginValue)
	if len(allowedOrigins) == 0 {
		return Config{}, errors.New("parse CORS_ALLOWED_ORIGINS: at least one origin is required")
	}

	providerBaseURL := firstValue("LLM_PROVIDER_BASE_URL", "API_URL")
	if providerBaseURL != "" {
		if err := absoluteHTTPURL("LLM_PROVIDER_BASE_URL (or API_URL alias)", providerBaseURL); err != nil {
			return Config{}, err
		}
		providerBaseURL = strings.TrimRight(providerBaseURL, "/")
	}
	providerKind, err := nonEmptyValue("LLM_PROVIDER_KIND")
	if err != nil {
		return Config{}, err
	}
	defaultModel, err := nonEmptyValue("LLM_DEFAULT_MODEL")
	if err != nil {
		return Config{}, err
	}
	if _, ok := os.LookupEnv("LLM_PROVIDER_API_KEY"); !ok {
		if _, aliasOK := os.LookupEnv("API_KEY"); !aliasOK {
			return Config{}, errors.New("parse LLM_PROVIDER_API_KEY: environment variable is required")
		}
	}

	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	requestTimeout, err := duration("HTTP_REQUEST_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	databaseMaxLifetime, err := duration("DATABASE_MAX_LIFETIME")
	if err != nil {
		return Config{}, err
	}
	databaseMaxIdleTime, err := duration("DATABASE_MAX_IDLE_TIME")
	if err != nil {
		return Config{}, err
	}
	databaseHealthTimeout, err := duration("DATABASE_HEALTH_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	redisHealthTimeout, err := duration("REDIS_HEALTH_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := duration("JWT_ACCESS_TTL")
	if err != nil {
		return Config{}, err
	}
	refreshTokenTTL, err := duration("AUTH_REFRESH_TTL")
	if err != nil {
		return Config{}, err
	}
	recentAuthTTL, err := duration("AUTH_RECENT_TTL")
	if err != nil {
		return Config{}, err
	}

	redisPrefix, err := nonEmptyValue("REDIS_PREFIX")
	if err != nil {
		return Config{}, err
	}
	jwtIssuer, err := nonEmptyValue("JWT_ISSUER")
	if err != nil {
		return Config{}, err
	}
	jwtAudience, err := nonEmptyValue("JWT_AUDIENCE")
	if err != nil {
		return Config{}, err
	}
	jwtActiveKeyID, err := nonEmptyValue("JWT_ACTIVE_KID")
	if err != nil {
		return Config{}, err
	}
	jwtPrivateKeyPath, err := value("JWT_PRIVATE_KEY_PATH")
	if err != nil {
		return Config{}, err
	}
	termsVersion, err := nonEmptyValue("TERMS_VERSION")
	if err != nil {
		return Config{}, err
	}
	privacyVersion, err := nonEmptyValue("PRIVACY_VERSION")
	if err != nil {
		return Config{}, err
	}
	cookieDomain, err := value("COOKIE_DOMAIN")
	if err != nil {
		return Config{}, err
	}
	cookieSameSite, err := nonEmptyValue("COOKIE_SAME_SITE")
	if err != nil {
		return Config{}, err
	}
	bootstrapEmails, err := value("BOOTSTRAP_ADMIN_EMAILS")
	if err != nil {
		return Config{}, err
	}
	otelServiceName, err := nonEmptyValue("OTEL_SERVICE_NAME")
	if err != nil {
		return Config{}, err
	}
	otelEndpoint, err := value("OTEL_EXPORTER_OTLP_ENDPOINT")
	if err != nil {
		return Config{}, err
	}
	metricsPath, err := nonEmptyValue("METRICS_PATH")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Runtime: Runtime{
			Environment:     environment,
			APIAddress:      fmt.Sprintf(":%d", port),
			WebURL:          strings.TrimRight(webURL, "/"),
			ShutdownTimeout: shutdownTimeout,
			RequestTimeout:  requestTimeout,
			MaxBodyBytes:    maxBodyBytes,
			Maintenance:     maintenance,
			TrustedProxies:  trustedProxies,
			AllowedOrigins:  allowedOrigins,
		},
		Database: Database{
			URL:              databaseURL,
			MaxConnections:   int32(maxConnections),
			MinConnections:   int32(minConnections),
			MaxLifetime:      databaseMaxLifetime,
			MaxIdleTime:      databaseMaxIdleTime,
			HealthTimeout:    databaseHealthTimeout,
			MigrateOnStartup: migrateOnStartup,
		},
		Redis: Redis{
			URL:           redisURL,
			Prefix:        redisPrefix,
			HealthTimeout: redisHealthTimeout,
		},
		OAuth: OAuth{
			Enabled:      oauthEnabled,
			TestMode:     oauthTestMode,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			CallbackURL:  callbackURL,
			TestEmail:    strings.ToLower(strings.TrimSpace(oauthTestEmail)),
		},
		Auth: Auth{
			Issuer:          jwtIssuer,
			Audience:        jwtAudience,
			ActiveKeyID:     jwtActiveKeyID,
			PrivateKeyPath:  jwtPrivateKeyPath,
			AccessTokenTTL:  accessTokenTTL,
			RefreshTokenTTL: refreshTokenTTL,
			RecentAuthTTL:   recentAuthTTL,
			TermsVersion:    termsVersion,
			PrivacyVersion:  privacyVersion,
		},
		Cookies: Cookies{
			SigningKey: cookieKey,
			Domain:     cookieDomain,
			Secure:     cookieSecure,
			SameSite:   cookieSameSite,
		},
		Provider: Provider{
			Kind:         providerKind,
			BaseURL:      providerBaseURL,
			APIKey:       firstValue("LLM_PROVIDER_API_KEY", "API_KEY"),
			DefaultModel: defaultModel,
		},
		Admin: Admin{BootstrapEmails: normalizedSet(bootstrapEmails)},
		Telemetry: Telemetry{
			ServiceName:  otelServiceName,
			OTLPEndpoint: otelEndpoint,
			MetricsPath:  metricsPath,
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
	if err := absoluteHTTPURL("JWT_ISSUER", c.Auth.Issuer); err != nil {
		return err
	}
	for _, origin := range c.Runtime.AllowedOrigins {
		if err := absoluteHTTPURL("CORS_ALLOWED_ORIGINS", origin); err != nil {
			return err
		}
	}
	if c.OAuth.TestMode {
		if c.Runtime.Environment == "production" {
			return errors.New("validate OAUTH_TEST_MODE: forbidden in production")
		}
		if c.OAuth.ClientID != "" || c.OAuth.ClientSecret != "" {
			return errors.New("validate OAUTH_TEST_MODE: cannot be combined with Google credentials")
		}
		if c.OAuth.TestEmail == "" || !strings.Contains(c.OAuth.TestEmail, "@") {
			return errors.New("validate OAUTH_TEST_EMAIL: valid email required in test mode")
		}
	}
	switch c.Provider.Kind {
	case "fake":
	case "openai-compatible":
		if c.Provider.BaseURL == "" {
			return errors.New("validate LLM_PROVIDER_BASE_URL: required for openai-compatible provider")
		}
		if c.Provider.APIKey == "" {
			return errors.New("validate LLM_PROVIDER_API_KEY: required for openai-compatible provider")
		}
	default:
		return errors.New("validate LLM_PROVIDER_KIND: must be fake or openai-compatible")
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

func loadEnvironmentFile() error {
	if configured := os.Getenv("GLAZZ_ENV_FILE"); configured != "" {
		if err := godotenv.Load(configured); err != nil {
			return fmt.Errorf("load GLAZZ_ENV_FILE: %w", err)
		}
		return nil
	}
	directory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve environment directory: %w", err)
	}
	for {
		if _, markerErr := os.Stat(filepath.Join(directory, "go.work")); markerErr == nil {
			envPath := filepath.Join(directory, ".env")
			if loadErr := godotenv.Load(envPath); loadErr != nil &&
				!errors.Is(loadErr, os.ErrNotExist) {
				return fmt.Errorf("load environment file: %w", loadErr)
			}
			return nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load environment file: %w", err)
	}
	return nil
}

func cookieSigningKey() ([]byte, error) {
	raw, err := nonEmptyValue("COOKIE_SIGNING_KEY")
	if err != nil {
		return nil, err
	}
	key, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(key) < 32 {
		return nil, errors.New("parse COOKIE_SIGNING_KEY: must be base64url without padding and decode to at least 32 bytes")
	}
	return key, nil
}

func duration(key string) (time.Duration, error) {
	raw, err := nonEmptyValue(key)
	if err != nil {
		return 0, err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("parse %s: must be a positive duration", key)
	}
	return parsed, nil
}

func integer(key string, minimum, maximum int) (int, error) {
	raw, err := nonEmptyValue(key)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("parse %s: must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func integer64(key string, minimum, maximum int64) (int64, error) {
	raw, err := nonEmptyValue(key)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("parse %s: must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func boolean(key string) (bool, error) {
	raw, err := nonEmptyValue(key)
	if err != nil {
		return false, err
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

func value(key string) (string, error) {
	result, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("parse %s: environment variable is required", key)
	}
	return result, nil
}

func nonEmptyValue(key string) (string, error) {
	result, err := value(key)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("parse %s: value must not be empty", key)
	}
	return result, nil
}

func firstValue(primary, alias string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	return os.Getenv(alias)
}
