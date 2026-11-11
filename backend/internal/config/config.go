package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/lib/pq"
	"github.com/spf13/viper"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/middleware"
)

// MinJWTSecretBytes is the minimum JWT_SECRET length: HS256 needs a key of at
// least 256 bits to keep its security margin.
const MinJWTSecretBytes = 32

// logLevels are the accepted LOG_LEVEL values. The list is closed on purpose:
// zerolog would also take "disabled", "panic" or numbers, which hide errors.
var logLevels = []string{"trace", "debug", "info", "warn", "error"}

// secureSSLModes are the sslmode values accepted outside development/test
// (ADR 0004 § Decisión 2): every one of them encrypts the connection.
var secureSSLModes = []pq.SSLMode{pq.SSLModeRequire, pq.SSLModeVerifyCA, pq.SSLModeVerifyFull}

type Config struct {
	Port          string
	Env           string
	DatabaseURL   string
	JWTSecret     string
	EncryptionKey string

	CORSOrigins string
	// TrustedProxies: comma-separated CIDRs or IPs, e.g. "10.0.0.0/8,172.16.0.0/12".
	// Empty (default) trusts no proxy; an invalid entry fails Load.
	TrustedProxies string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	FrontendBaseURL string

	// LogLevel (trace|debug|info|warn|error, case-insensitive) overrides the
	// logger's default level (debug in development, info otherwise); empty
	// keeps the default and any other value fails Load. Read through viper so
	// a value in .env applies too.
	LogLevel string
}

// Load reads configuration from environment / .env file.
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = viper.ReadInConfig()

	cfg := &Config{
		Port:            viper.GetString("PORT"),
		Env:             viper.GetString("ENV"),
		DatabaseURL:     viper.GetString("DATABASE_URL"),
		JWTSecret:       viper.GetString("JWT_SECRET"),
		EncryptionKey:   viper.GetString("ENCRYPTION_KEY"),
		SMTPHost:        viper.GetString("SMTP_HOST"),
		SMTPPort:        viper.GetInt("SMTP_PORT"),
		SMTPUser:        viper.GetString("SMTP_USER"),
		SMTPPassword:    viper.GetString("SMTP_PASSWORD"),
		SMTPFrom:        viper.GetString("SMTP_FROM"),
		CORSOrigins:     viper.GetString("CORS_ORIGINS"),
		TrustedProxies:  viper.GetString("TRUSTED_PROXIES"),
		FrontendBaseURL: viper.GetString("FRONTEND_BASE_URL"),
		LogLevel:        viper.GetString("LOG_LEVEL"),
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < MinJWTSecretBytes {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d bytes (got %d)", MinJWTSecretBytes, len(cfg.JWTSecret))
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}
	// Same rule as crypto.NewCipher, checked before connecting to the
	// database. The decode error is not wrapped: it quotes part of the key.
	if _, err := crypto.NewCipher(cfg.EncryptionKey); err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 64 hex characters (a 32-byte key)")
	}
	if _, err := middleware.ParseTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	if cfg.LogLevel != "" && !slices.Contains(logLevels, cfg.LogLevel) {
		return nil, fmt.Errorf("LOG_LEVEL must be one of %s or empty (got %q)", strings.Join(logLevels, "|"), cfg.LogLevel)
	}

	// CORS must be configured explicitly unless ENV says, exactly,
	// development or test: the empty-list fallback to http://localhost:3000
	// (middleware.CORS) is only meant for local work, and an unset or
	// misspelled ENV must fail closed rather than run with it.
	localEnv := cfg.Env == "development" || cfg.Env == "test"
	if !localEnv && strings.TrimSpace(cfg.CORSOrigins) == "" {
		return nil, fmt.Errorf("CORS_ORIGINS is required unless ENV is development or test (ENV=%q)", cfg.Env)
	}

	if !localEnv {
		if err := requireSecureSSLMode(cfg.DatabaseURL); err != nil {
			return nil, err
		}
	}

	// Same rule for the e-mail settings: outside development/test the worker
	// must be able to send, and the links in the e-mails must not fall back
	// to http://localhost:3000 (cmd/server).
	if !localEnv {
		switch {
		case strings.TrimSpace(cfg.SMTPHost) == "":
			return nil, fmt.Errorf("SMTP_HOST is required unless ENV is development or test (ENV=%q)", cfg.Env)
		case cfg.SMTPPort <= 0:
			return nil, fmt.Errorf("SMTP_PORT must be a positive port unless ENV is development or test (ENV=%q)", cfg.Env)
		case strings.TrimSpace(cfg.SMTPFrom) == "":
			return nil, fmt.Errorf("SMTP_FROM is required unless ENV is development or test (ENV=%q)", cfg.Env)
		case strings.TrimSpace(cfg.FrontendBaseURL) == "":
			return nil, fmt.Errorf("FRONTEND_BASE_URL is required unless ENV is development or test (ENV=%q)", cfg.Env)
		}
	}

	if cfg.FrontendBaseURL != "" {
		// The e-mail links are built as FrontendBaseURL + "/auth/...", so a
		// trailing slash would produce "//auth/...".
		cfg.FrontendBaseURL = strings.TrimRight(strings.TrimSpace(cfg.FrontendBaseURL), "/")
		if u, err := url.ParseRequestURI(cfg.FrontendBaseURL); err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("FRONTEND_BASE_URL must be a valid URL (e.g. https://example.com)")
		}
	}

	return cfg, nil
}

// requireSecureSSLMode checks the sslmode that lib/pq will actually use for
// dsn (URL or key=value; a PGSSLMODE in the environment counts when the DSN
// does not set it). It must be one of secureSSLModes: an unset sslmode fails
// too, so production never depends on the driver default. The errors never
// quote the DSN, which carries the password.
func requireSecureSSLMode(dsn string) error {
	pgCfg, err := pq.NewConfig(dsn)
	if err != nil {
		// pq's error is not wrapped: it may quote the DSN.
		return fmt.Errorf("DATABASE_URL (with the PG* environment variables) is not a valid PostgreSQL connection string")
	}
	if !slices.Contains(secureSSLModes, pgCfg.SSLMode) {
		return fmt.Errorf("DATABASE_URL must set sslmode=require, verify-ca or verify-full (or PGSSLMODE, if the URL does not set it) unless ENV is development or test (got %q)", pgCfg.SSLMode)
	}
	return nil
}
