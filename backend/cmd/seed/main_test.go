package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitValues(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", `'a', 'b', 3`, []string{"a", "b", "3"}},
		{"escaped quote", `'D''ANGELO', 'x'`, []string{"D'ANGELO", "x"}},
		{"empty string", `'', 'x', ''`, []string{"", "x", ""}},
		{"only escaped quote", `''''`, []string{"'"}},
		{"comma inside quotes", `'Pérez, Juan', 2`, []string{"Pérez, Juan", "2"}},
		{"escape next to comma", `'O''', 'x'`, []string{"O'", "x"}},
		{"unicode", `'ÑAÑEZ', 'María José', 'Güemes'`, []string{"ÑAÑEZ", "María José", "Güemes"}},
		{"spaces outside quotes ignored", `  'a'  ,   7  ,NOW()`, []string{"a", "7", "NOW()"}},
		{"spaces inside quotes kept", `' a b '`, []string{" a b "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitValues(tc.in)
			if err != nil {
				t.Fatalf("splitValues(%q) error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitValues(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitValuesUnterminated(t *testing.T) {
	if _, err := splitValues(`'abc, 1`); err == nil {
		t.Fatal("expected error for unterminated quote")
	}
	if _, err := splitValues(`'it''s`); err == nil {
		t.Fatal("expected error: escaped quote does not close the string")
	}
}

const hdr = `INSERT INTO personas (dni, nombres, apellido_paterno, apellido_materno, grado_id, especialidad_id, institucion_id, telefono, email, estado, created_at, updated_at) VALUES `

func TestParseSeed(t *testing.T) {
	input := strings.Join([]string{
		"-- comment",
		"",
		hdr + "('11111111', 'ANA', 'D''ANGELO', 'RÍOS', 6, 28, 1, '999', 'Ana@X.com', 'activo', NOW(), NOW()) ON CONFLICT (dni) DO NOTHING;", // 3
		"INSERT INTO personas garbage;", // 4
		hdr + "('22222222', 'B', 'C', 'D', x, 28, 1, '', '', 'activo', NOW(), NOW()) ON CONFLICT (dni) DO NOTHING;", // 5
		hdr + "('33333333', 'B', 'C', 'D', 1, 2, 3, '', '') ON CONFLICT (dni) DO NOTHING;",                          // 6
		hdr + "('44444444', 'B, 'C', 'D', 1, 2, 3, '', '', 'activo', NOW(), NOW()) ON CONFLICT (dni) DO NOTHING;",   // 7
		"DELETE FROM personas;", // 8
		"INSERT INTO personas (nombres, dni, apellido_paterno, apellido_materno, grado_id, especialidad_id, institucion_id, telefono, email, estado) VALUES ('a','1','b','c',1,2,3,'','','activo') ON CONFLICT (dni) DO NOTHING;", // 9
		"COMMIT;",
	}, "\n")

	recs, skipped, err := parseSeed(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	r := recs[0]
	if r.Line != 3 || r.DNI != "11111111" || r.ApellidoP != "D'ANGELO" || r.ApellidoM != "RÍOS" ||
		r.GradoID != 6 || r.EspecialidadID != 28 || r.InstitucionID != 1 || r.Email != "Ana@X.com" || r.Estado != "activo" {
		t.Fatalf("bad record: %+v", r)
	}
	var lines []int
	for _, s := range skipped {
		lines = append(lines, s.Line)
		for _, pii := range []string{"22222222", "33333333", "44444444"} {
			if strings.Contains(s.Reason, pii) {
				t.Fatalf("skip reason leaks PII: %q", s.Reason)
			}
		}
	}
	if want := []int{4, 5, 6, 7, 8, 9}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("skipped lines = %v, want %v (%+v)", lines, want, skipped)
	}
}
