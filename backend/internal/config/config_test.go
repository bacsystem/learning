package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("0", 64))
	t.Setenv("ENV", "development")
	t.Setenv("CORS_ORIGINS", "")
	t.Setenv("FRONTEND_BASE_URL", "")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("TRUSTED_PROXIES", "")
}

// setProductionEnv is setBaseEnv plus everything a non-development ENV needs.
func setProductionEnv(t *testing.T) {
	t.Helper()
	setBaseEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://u:p@db.internal:5432/app?sslmode=require")
	t.Setenv("CORS_ORIGINS", "https://app.example.com")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_FROM", "no-reply@example.com")
	t.Setenv("FRONTEND_BASE_URL", "https://app.example.com")
}

func TestLoad_OK(t *testing.T) {
	setBaseEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default port %q", cfg.Port)
	}
}

// H51: JWT_SECRET must be at least 32 bytes.
func TestLoad_ShortJWTSecret(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("JWT_SECRET", strings.Repeat("s", 31))
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("want JWT_SECRET length error, got %v", err)
	}
}

// H51: production must not silently fall back to localhost CORS.
func TestLoad_ProductionRequiresCORSOrigins(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "production")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CORS_ORIGINS") {
		t.Fatalf("want CORS_ORIGINS error, got %v", err)
	}
	setProductionEnv(t)
	if _, err := Load(); err != nil {
		t.Fatalf("production with CORS_ORIGINS: %v", err)
	}
}

// H93: the localhost CORS fallback is allowed only when ENV explicitly says
// development or test; any other value (unset, a typo, "staging", "prod",
// "Production") requires CORS_ORIGINS.
func TestLoad_CORSOriginsRequiredUnlessDevelopmentOrTest(t *testing.T) {
	for _, env := range []string{"", "prod", "staging", "Production", "dev"} {
		setBaseEnv(t)
		t.Setenv("ENV", env)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CORS_ORIGINS") {
			t.Errorf("ENV=%q without CORS_ORIGINS: want CORS_ORIGINS error, got %v", env, err)
		}
	}
	for _, env := range []string{"development", "test"} {
		setBaseEnv(t)
		t.Setenv("ENV", env)
		if _, err := Load(); err != nil {
			t.Errorf("ENV=%q without CORS_ORIGINS: %v", env, err)
		}
	}
}

func TestLoad_InvalidFrontendBaseURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("FRONTEND_BASE_URL", "not a url")
	if _, err := Load(); err == nil {
		t.Fatal("want error for invalid FRONTEND_BASE_URL")
	}
}

// H125: outside development/test the SMTP settings and FRONTEND_BASE_URL are
// required (no silent fallback to http://localhost:3000 in the e-mail links).
func TestLoad_ProductionRequiresSMTPAndFrontendURL(t *testing.T) {
	cases := []struct {
		key, value, wantErr string
	}{
		{"SMTP_HOST", "", "SMTP_HOST"},
		{"SMTP_HOST", "   ", "SMTP_HOST"},
		{"SMTP_PORT", "", "SMTP_PORT"},
		{"SMTP_PORT", "0", "SMTP_PORT"},
		{"SMTP_PORT", "-1", "SMTP_PORT"},
		{"SMTP_FROM", "", "SMTP_FROM"},
		{"FRONTEND_BASE_URL", "", "FRONTEND_BASE_URL"},
	}
	for _, env := range []string{"production", "staging", ""} {
		for _, c := range cases {
			setProductionEnv(t)
			t.Setenv("ENV", env)
			t.Setenv(c.key, c.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("ENV=%q %s=%q: want %s error, got %v", env, c.key, c.value, c.wantErr, err)
			}
		}
	}

	setProductionEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("complete production config: %v", err)
	}
	if cfg.SMTPPort != 587 || cfg.FrontendBaseURL != "https://app.example.com" {
		t.Errorf("unexpected cfg %+v", cfg)
	}

	// development and test keep working without SMTP or FRONTEND_BASE_URL.
	for _, env := range []string{"development", "test"} {
		setBaseEnv(t)
		t.Setenv("ENV", env)
		if _, err := Load(); err != nil {
			t.Errorf("ENV=%q without SMTP/FRONTEND_BASE_URL: %v", env, err)
		}
	}
}

// H125: a trailing slash in FRONTEND_BASE_URL would build links such as
// https://app.example.com//auth/..., so Load normalizes it away.
func TestLoad_FrontendBaseURLTrailingSlashNormalized(t *testing.T) {
	for _, in := range []string{"https://app.example.com/", "https://app.example.com//", "https://app.example.com/app/"} {
		setProductionEnv(t)
		t.Setenv("FRONTEND_BASE_URL", in)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("FRONTEND_BASE_URL=%q: %v", in, err)
		}
		if strings.HasSuffix(cfg.FrontendBaseURL, "/") {
			t.Errorf("FRONTEND_BASE_URL=%q: got %q, want no trailing slash", in, cfg.FrontendBaseURL)
		}
	}
	setBaseEnv(t)
	t.Setenv("FRONTEND_BASE_URL", "/")
	if _, err := Load(); err == nil {
		t.Error("FRONTEND_BASE_URL=\"/\": want invalid URL error")
	}
}

// H145: LOG_LEVEL is read through Config (viper), so a value in .env reaches
// the logger even when it is not exported to the process environment (go run
// outside compose).
func TestLoad_LogLevelFromDotEnv(t *testing.T) {
	setBaseEnv(t)
	if err := os.Unsetenv("LOG_LEVEL"); err != nil { // only the .env file provides it
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("LOG_LEVEL=warn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Cleanup(viper.Reset)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel %q, want warn", cfg.LogLevel)
	}
}

// H-2 (PR #97): ENCRYPTION_KEY is validated in Load, before the database
// connects, with an error that names the variable and never echoes the key.
func TestLoad_EncryptionKeyFormat(t *testing.T) {
	cases := []struct {
		name, key string
		wantErr   bool
	}{
		{"64 hex chars", strings.Repeat("ab", 32), false},
		{"upper-case hex", strings.Repeat("AB", 32), false},
		{"too short", strings.Repeat("0", 62), true},
		{"too long", strings.Repeat("0", 66), true},
		{"odd length", strings.Repeat("0", 63), true},
		{"not hex", strings.Repeat("z", 64), true},
		{"surrounding spaces", " " + strings.Repeat("0", 64) + " ", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("ENCRYPTION_KEY", c.key)
			_, err := Load()
			if !c.wantErr {
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
				t.Fatalf("want ENCRYPTION_KEY error, got %v", err)
			}
			if strings.Contains(err.Error(), strings.TrimSpace(c.key)) {
				t.Errorf("error echoes the key: %v", err)
			}
		})
	}
}

// H-3 (PR #97): an invalid TRUSTED_PROXIES entry fails Load instead of being
// dropped; empty or blank means no proxy.
func TestLoad_TrustedProxies(t *testing.T) {
	cases := []struct {
		name, value string
		wantErr     bool
	}{
		{"unset", "", false},
		{"blank", "   ", false},
		{"CIDR list", "10.0.0.0/8, 172.16.0.0/12", false},
		{"single IPv4 and IPv6", "10.1.1.1,2001:db8::1", false},
		{"trailing comma", "10.0.0.0/8,", false},
		{"typo in IP", "10.0.0.256", true},
		{"bad mask", "10.0.0.0/33", true},
		{"hostname", "proxy.internal", true},
		{"one bad entry among good ones", "10.0.0.0/8,nope", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("TRUSTED_PROXIES", c.value)
			_, err := Load()
			if c.wantErr && (err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES")) {
				t.Fatalf("want TRUSTED_PROXIES error, got %v", err)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("Load: %v", err)
			}
		})
	}
}

// H-3 (PR #97): LOG_LEVEL accepts only the closed list, case-insensitive and
// trimmed; anything else fails Load instead of falling back silently.
func TestLoad_LogLevel(t *testing.T) {
	cases := []struct {
		value, want string
		wantErr     bool
	}{
		{"", "", false},
		{"debug", "debug", false},
		{" INFO ", "info", false},
		{"Warn", "warn", false},
		{"trace", "trace", false},
		{"error", "error", false},
		{"warning", "", true},
		{"disabled", "", true},
		{"panic", "", true},
		{"5", "", true},
	}
	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("LOG_LEVEL", c.value)
			cfg, err := Load()
			if c.wantErr {
				if err == nil || !strings.Contains(err.Error(), "LOG_LEVEL") {
					t.Fatalf("want LOG_LEVEL error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.LogLevel != c.want {
				t.Errorf("LogLevel %q, want %q", cfg.LogLevel, c.want)
			}
		})
	}
}

// H-5 (PR #97, ADR 0004 § Decisión 2): outside development/test the effective
// sslmode of DATABASE_URL must encrypt; the error never quotes the DSN.
func TestLoad_DatabaseSSLMode(t *testing.T) {
	const secret = "s3cr3t-pass"
	cases := []struct {
		name, env, dsn, pgsslmode string
		wantErr                   bool
	}{
		{"URL require", "production", "postgres://u:" + secret + "@db:5432/app?sslmode=require", "", false},
		{"Railway URL with other params", "production", "postgresql://u:" + secret + "@db.railway.internal:5432/railway?connect_timeout=10&sslmode=require", "", false},
		{"verify-full", "production", "postgres://u:" + secret + "@db/app?sslmode=verify-full", "", false},
		{"key=value verify-ca", "production", "host=db user=u password=" + secret + " dbname=app sslmode=verify-ca", "", false},
		{"unset", "production", "postgres://u:" + secret + "@db/app", "", true},
		{"disable", "production", "postgres://u:" + secret + "@db/app?sslmode=disable", "", true},
		{"prefer", "staging", "postgres://u:" + secret + "@db/app?sslmode=prefer", "", true},
		{"allow, key=value", "", "host=db user=u password=" + secret + " sslmode=allow", "", true},
		{"PGSSLMODE disable without DSN sslmode", "production", "postgres://u:" + secret + "@db/app", "disable", true},
		{"DSN require wins over PGSSLMODE", "production", "postgres://u:" + secret + "@db/app?sslmode=require", "disable", false},
		{"unknown sslmode", "production", "postgres://u:" + secret + "@db/app?sslmode=bogus", "", true},
		{"malformed URL", "production", "postgres://u:" + secret + "@db:notaport/app?sslmode=require", "", true},
		{"development keeps disable", "development", "postgres://u:" + secret + "@db/app?sslmode=disable", "", false},
		{"test keeps unset", "test", "postgres://u:" + secret + "@db/app", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setProductionEnv(t)
			t.Setenv("ENV", c.env)
			t.Setenv("DATABASE_URL", c.dsn)
			t.Setenv("PGSSLMODE", c.pgsslmode) // restores the variable after the test
			if c.pgsslmode == "" {
				// lib/pq rejects an empty PGSSLMODE, so "unset" must really unset it.
				if err := os.Unsetenv("PGSSLMODE"); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Load()
			if !c.wantErr {
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
				t.Fatalf("want DATABASE_URL error, got %v", err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error quotes the DSN: %v", err)
			}
		})
	}
}
