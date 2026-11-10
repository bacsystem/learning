package auth

import (
	"context"
	"encoding/json"
	"testing"
)

// ── H91: audit detalle is always valid JSON ─────────────────────────────────

// An observación with control characters, NUL or invalid UTF-8 must still
// produce an audit row (detalle is jsonb): building it with %q yielded Go
// escapes ("\x01", "\xff") that are not JSON, the insert failed and the state
// change went unaudited.
func TestUpdateEstado_ObservacionWithControlCharsAndInvalidUTF8_Audited(t *testing.T) {
	f := newFixture(t)
	adminID, _ := f.activeUser("audadm", "admin")
	pid, _ := f.persona(f.email("audtgt"))
	target := f.user(pid, f.username("audtgt"), "Password123!", "usuario", "pendiente")

	obs := "ok\x01ctrl\x00nul\xffbad\"quote\\"
	if _, err := f.svc.UpdateEstado(context.Background(), target, UpdateUsuarioEstadoRequest{Estado: "activo", Observacion: obs}, adminID, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	var detalle []byte
	if err := f.db.QueryRow(`SELECT detalle FROM audit_log WHERE usuario_id=$1 AND accion='activo' AND entidad='usuarios' AND entidad_id=$2`,
		adminID, target).Scan(&detalle); err != nil {
		t.Fatalf("audit row not written: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(detalle, &got); err != nil {
		t.Fatalf("detalle is not JSON: %v (%s)", err, detalle)
	}
	if got["observacion"] != "ok\x01ctrlnul�bad\"quote\\" {
		t.Fatalf("observacion = %q", got["observacion"])
	}
}
