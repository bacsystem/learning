package logger

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init configures the GLOBAL zerolog logger (github.com/rs/zerolog/log), which
// is what the whole codebase logs through. Call once at startup.
//
//   - development: human-readable console output on stderr, level debug.
//   - any other env: JSON on stdout, level info.
//
// level (trace|debug|info|warn|error; config.Config.LogLevel, i.e. LOG_LEVEL
// read through viper so .env applies too) overrides the default level; empty
// or invalid keeps the default.
func Init(env, level string) {
	InitWithWriter(env, level, nil)
}

// InitWithWriter is Init with an explicit level and output (nil = default
// output for env). Exposed for tests.
func InitWithWriter(env, level string, out io.Writer) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	lvl := zerolog.InfoLevel
	if env == "development" {
		lvl = zerolog.DebugLevel
	}
	if level != "" {
		if parsed, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level))); err == nil && parsed != zerolog.NoLevel {
			lvl = parsed
		}
	}
	zerolog.SetGlobalLevel(lvl)

	if out == nil {
		if env == "development" {
			out = zerolog.ConsoleWriter{Out: os.Stderr}
		} else {
			out = os.Stdout
		}
	}
	log.Logger = zerolog.New(out).With().Timestamp().Logger()
}
