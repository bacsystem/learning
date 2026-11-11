package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestParsePage(t *testing.T) {
	cases := []struct {
		in   string
		want int
		bad  bool
	}{
		{"", 1, false},
		{"1", 1, false},
		{"7", 7, false},
		{"5000", 1000, false},
		{"99999999999999999999", 1000, false}, // beyond int64: capped, not page 1
		{"0", 1, true},
		{"-3", 1, true},
		{"-99999999999999999999", 1, true},
		{"abc", 1, true},
		{"1.5", 1, true},
	}
	for _, c := range cases {
		got, err := ParsePage(c.in, 1000)
		if got != c.want || errors.Is(err, ErrBadNumber) != c.bad {
			t.Errorf("ParsePage(%q) = %d, %v; want %d, bad=%v", c.in, got, err, c.want, c.bad)
		}
	}
}

func TestParsePerPage(t *testing.T) {
	cases := []struct {
		in   string
		want int
		bad  bool
	}{
		{"", 20, false},
		{"5", 5, false},
		{"101", 100, false},
		{"99999999999999999999", 100, false},
		{"0", 20, true},
		{"x", 20, true},
	}
	for _, c := range cases {
		got, err := ParsePerPage(c.in, 20, 100)
		if got != c.want || errors.Is(err, ErrBadNumber) != c.bad {
			t.Errorf("ParsePerPage(%q) = %d, %v; want %d, bad=%v", c.in, got, err, c.want, c.bad)
		}
	}
}

func TestInternalError(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	rr := httptest.NewRecorder()
	InternalError(rr, "pkg.op", errors.New("boom"))
	if rr.Code != http.StatusInternalServerError || rr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("got %d %q", rr.Code, rr.Header().Get("Content-Type"))
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body["error"] != GenericInternalMsg {
		t.Fatalf("body %q", rr.Body)
	}
	if out := buf.String(); !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"op":"pkg.op"`) || !strings.Contains(out, "boom") {
		t.Fatalf("log %q", out)
	}

	rr = httptest.NewRecorder()
	InternalErrorMsg(rr, "pkg.op2", errors.New("x"), "Error al obtener.")
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if rr.Code != http.StatusInternalServerError || body["error"] != "Error al obtener." {
		t.Fatalf("custom msg: %d %q", rr.Code, rr.Body)
	}
}

func TestJSONErrorCode(t *testing.T) {
	cases := []struct {
		name    string
		detalle map[string]any
		want    string
	}{
		{"sin detalle", nil, `{"error":"tema_duplicado","message":"Ya existe."}`},
		{"detalle vacío se omite", map[string]any{}, `{"error":"tema_duplicado","message":"Ya existe."}`},
		{"con detalle", map[string]any{"campo": "nombre"},
			`{"error":"tema_duplicado","message":"Ya existe.","detalle":{"campo":"nombre"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			JSONErrorCode(rr, http.StatusUnprocessableEntity, "tema_duplicado", "Ya existe.", c.detalle)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d", rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type = %q", ct)
			}
			if got := strings.TrimSpace(rr.Body.String()); got != c.want {
				t.Errorf("body = %s, want %s", got, c.want)
			}
		})
	}
}

func TestInternalErrorCode(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	rr := httptest.NewRecorder()
	InternalErrorCode(rr, "temas.crear", errors.New("boom"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != CodeInternal || body["message"] != GenericInternalMsg {
		t.Errorf("body = %v", body)
	}
	if !strings.Contains(buf.String(), `"op":"temas.crear"`) || !strings.Contains(buf.String(), "boom") {
		t.Errorf("log = %s", buf.String())
	}
}
