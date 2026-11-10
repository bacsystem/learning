package testimonios

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(NewService(NewRepository(dbtest.Open(t), nil)), nil)
}

func TestGetPublicos_Empty(t *testing.T) {
	h := testHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/testimonios/publico", nil)
	rr := httptest.NewRecorder()
	h.GetPublicos(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var list []TestimonioPublicoDTO
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestUpdateEstado_InvalidID(t *testing.T) {
	h := testHandler(t)
	body := bytes.NewBufferString(`{"estado":"aprobado"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/testimonios/abc/estado", body)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.UpdateEstado(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
