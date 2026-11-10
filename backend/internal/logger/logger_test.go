package logger

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func restoreGlobal(t *testing.T) {
	prevLogger, prevLevel := log.Logger, zerolog.GlobalLevel()
	t.Cleanup(func() {
		log.Logger = prevLogger
		zerolog.SetGlobalLevel(prevLevel)
	})
}

// H29: Init must configure the global logger used by the code (zerolog/log).
func TestInit_ConfiguresGlobalLogger(t *testing.T) {
	restoreGlobal(t)
	var buf bytes.Buffer
	InitWithWriter("production", "", &buf)

	log.Info().Str("k", "v").Msg("hello")
	log.Debug().Msg("hidden at info level")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("want exactly one JSON line from the global logger, got %q (%v)", buf.String(), err)
	}
	if entry["message"] != "hello" || entry["k"] != "v" || entry["time"] == nil {
		t.Errorf("unexpected entry %v", entry)
	}
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("level %v, want info", zerolog.GlobalLevel())
	}
}

func TestInit_DevelopmentIsDebug(t *testing.T) {
	restoreGlobal(t)
	InitWithWriter("development", "", &bytes.Buffer{})
	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Errorf("level %v, want debug", zerolog.GlobalLevel())
	}
}

func TestInit_LogLevelOverride(t *testing.T) {
	restoreGlobal(t)
	InitWithWriter("production", "WARN", &bytes.Buffer{})
	if zerolog.GlobalLevel() != zerolog.WarnLevel {
		t.Errorf("level %v, want warn", zerolog.GlobalLevel())
	}
	InitWithWriter("production", "nonsense", &bytes.Buffer{})
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("invalid LOG_LEVEL: level %v, want default info", zerolog.GlobalLevel())
	}
}
