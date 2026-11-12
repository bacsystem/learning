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
	t.Setenv("ANTIFRAUD_HMAC_KEY", strings.Repeat("a", 64))
	t.Setenv("ANTIFRAUD_HMAC_KEY_PREVIOUS", "")
	t.Setenv("ENV", "development")
	t.Setenv("CORS_ORIGINS", "")
	t.Setenv("FRONTEND_BASE_URL", "")
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("IA_PROVEEDOR", "")
	t.Setenv("IA_API_KEY", "")
	t.Setenv("IA_MODELO", "")
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
	const (
		weak    = "sslmode=require"
		invalid = "connection string"
	)
	cases := []struct {
		name, env, dsn, pgsslmode string
		wantErr                   string // "" = Load succeeds
	}{
		{"URL require", "production", "postgres://u:" + secret + "@db:5432/app?sslmode=require", "", ""},
		{"Railway URL with other params", "production", "postgresql://u:" + secret + "@db.railway.internal:5432/railway?connect_timeout=10&sslmode=require", "", ""},
		{"verify-full", "production", "postgres://u:" + secret + "@db/app?sslmode=verify-full", "", ""},
		{"key=value verify-ca", "production", "host=db user=u password=" + secret + " dbname=app sslmode=verify-ca", "", ""},
		{"sslrootcert=system means verify-full", "production", "postgres://u:" + secret + "@db/app?sslrootcert=system", "", ""},
		{"unset", "production", "postgres://u:" + secret + "@db/app", "", weak},
		{"disable", "production", "postgres://u:" + secret + "@db/app?sslmode=disable", "", weak},
		{"prefer", "staging", "postgres://u:" + secret + "@db/app?sslmode=prefer", "", weak},
		{"allow, key=value", "", "host=db user=u password=" + secret + " sslmode=allow", "", weak},
		{"PGSSLMODE disable without DSN sslmode", "production", "postgres://u:" + secret + "@db/app", "disable", weak},
		{"PGSSLMODE require without DSN sslmode", "production", "postgres://u:" + secret + "@db/app", "require", ""},
		{"DSN require wins over PGSSLMODE", "production", "postgres://u:" + secret + "@db/app?sslmode=require", "disable", ""},
		{"unknown sslmode", "production", "postgres://u:" + secret + "@db/app?sslmode=bogus", "", invalid},
		{"malformed URL", "production", "postgres://u:" + secret + "@db:notaport/app?sslmode=require", "", invalid},
		{"development keeps disable", "development", "postgres://u:" + secret + "@db/app?sslmode=disable", "", ""},
		{"test keeps unset", "test", "postgres://u:" + secret + "@db/app", "", ""},
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
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("want DATABASE_URL error containing %q, got %v", c.wantErr, err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error quotes the DSN: %v", err)
			}
		})
	}
}

// SIA-E6-01 §3: empty disables the AI service; the double is accepted only
// with ENV development, test or preprod; anything else fails closed.
func TestLoad_IAProveedor(t *testing.T) {
	cases := []struct {
		name, env, proveedor string
		wantErr              bool
	}{
		{"empty disables it in production", "production", "", false},
		{"double in development", "development", "doble", false},
		{"double in test", "test", "doble", false},
		{"double in preprod", "preprod", "doble", false},
		{"double in production", "production", "doble", true},
		{"double with a misspelled ENV", "prod", "doble", true},
		{"double with ENV unset", "", "doble", true},
		{"unknown provider", "development", "openai", true},
		{"spaces are trimmed", "development", "  doble ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setProductionEnv(t)
			t.Setenv("ENV", c.env)
			t.Setenv("IA_PROVEEDOR", c.proveedor)
			t.Setenv("IA_API_KEY", "sk-secreto-no-se-imprime")
			cfg, err := Load()
			if c.wantErr {
				if err == nil || !strings.Contains(err.Error(), "IA_PROVEEDOR") {
					t.Fatalf("want IA_PROVEEDOR error, got %v", err)
				}
				if strings.Contains(err.Error(), "sk-secreto") {
					t.Errorf("error quotes IA_API_KEY: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.IAProveedor != strings.TrimSpace(c.proveedor) {
				t.Errorf("IAProveedor %q", cfg.IAProveedor)
			}
		})
	}
}

// ADR 0002: ANTIFRAUD_HMAC_KEY is required everywhere and checked at start.
func TestLoad_AntifraudKeys(t *testing.T) {
	otraClave := strings.Repeat("b", 64)
	tests := []struct {
		name    string
		vigente string
		previa  string
		jwt     string
		wantErr string // "" = ok; else the variable the error must name
	}{
		{"valid", strings.Repeat("a", 64), "", "", ""},
		{"valid with a previous key", strings.Repeat("a", 64), otraClave, "", ""},
		{"longer than 32 bytes is valid", strings.Repeat("a", 96), "", "", ""},
		{"surrounding spaces are trimmed", "  " + strings.Repeat("a", 64) + "\n", " " + strings.Repeat("b", 64) + " ", "", ""},
		{"equal to ENCRYPTION_KEY after trimming", " " + strings.Repeat("0", 64) + " ", "", "", "ENCRYPTION_KEY"},
		{"missing", "", "", "", "ANTIFRAUD_HMAC_KEY"},
		{"not hex", strings.Repeat("z", 64), "", "", "ANTIFRAUD_HMAC_KEY"},
		{"odd length hex", strings.Repeat("a", 63), "", "", "ANTIFRAUD_HMAC_KEY"},
		{"shorter than 32 bytes", strings.Repeat("a", 62), "", "", "ANTIFRAUD_HMAC_KEY"},
		{"equal to ENCRYPTION_KEY", strings.Repeat("0", 64), "", "", "ENCRYPTION_KEY"},
		{"equal to JWT_SECRET", strings.Repeat("a", 64), "", strings.Repeat("a", 64), "JWT_SECRET"},
		{"previous invalid", strings.Repeat("a", 64), "xyz", "", "ANTIFRAUD_HMAC_KEY_PREVIOUS"},
		{"previous too short", strings.Repeat("a", 64), strings.Repeat("b", 10), "", "ANTIFRAUD_HMAC_KEY_PREVIOUS"},
		{"previous equal to current", strings.Repeat("a", 64), strings.Repeat("a", 64), "", "ANTIFRAUD_HMAC_KEY_PREVIOUS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv("ANTIFRAUD_HMAC_KEY", tt.vigente)
			t.Setenv("ANTIFRAUD_HMAC_KEY_PREVIOUS", tt.previa)
			if tt.jwt != "" {
				t.Setenv("JWT_SECRET", tt.jwt)
			}
			cfg, err := Load()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Load: %v", err)
				}
				if len(cfg.AntifraudHMACKey) < MinAntifraudKeyBytes {
					t.Errorf("key has %d bytes", len(cfg.AntifraudHMACKey))
				}
				if (tt.previa == "") != (cfg.AntifraudHMACKeyPrevious == nil) {
					t.Errorf("previous key = %v for %q", cfg.AntifraudHMACKeyPrevious, tt.previa)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want an error naming %s, got %v", tt.wantErr, err)
			}
			for _, secret := range []string{tt.vigente, tt.previa} {
				if len(secret) > 8 && strings.Contains(err.Error(), secret) {
					t.Errorf("the error quotes a key: %v", err)
				}
			}
		})
	}
}

// The key is required in every environment, also in production.
func TestLoad_AntifraudKeyRequiredInProduction(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("ANTIFRAUD_HMAC_KEY", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "ANTIFRAUD_HMAC_KEY") {
		t.Fatalf("want ANTIFRAUD_HMAC_KEY error, got %v", err)
	}
}
