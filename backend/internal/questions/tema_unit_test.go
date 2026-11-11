package questions

import (
	"encoding/json"
	"testing"
)

func i64(v int64) *int64 { return &v }

func TestOptionalID_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		set   bool
		value *int64
		bad   bool
	}{
		{"ausente", `{}`, false, nil, false},
		{"null", `{"tema_id":null}`, true, nil, false},
		{"número", `{"tema_id":41}`, true, i64(41), false},
		{"cero llega tal cual (lo rechaza validarTema)", `{"tema_id":0}`, true, i64(0), false},
		{"texto", `{"tema_id":"41"}`, false, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var req struct {
				TemaID OptionalID `json:"tema_id"`
			}
			err := json.Unmarshal([]byte(c.body), &req)
			if (err != nil) != c.bad {
				t.Fatalf("err = %v, want bad=%v", err, c.bad)
			}
			if c.bad {
				return
			}
			if req.TemaID.Set != c.set || !sameID(req.TemaID.Value, c.value) {
				t.Errorf("got %+v (value %v), want set=%v value %v", req.TemaID, deref(req.TemaID.Value), c.set, deref(c.value))
			}
		})
	}
}

// The four cases of the tech-lead decision on T8 (plus a sent value with an
// asignatura change, which is still validated by validarTema).
func TestTemaTrasEditar(t *testing.T) {
	const asig, otraAsig = 7, 8
	actual := i64(41)
	cases := []struct {
		name    string
		enviado OptionalID
		actual  *int64
		nueva   int64
		want    *int64
	}{
		{"ausente conserva el tema", OptionalID{}, actual, asig, actual},
		{"ausente sin tema sigue sin tema", OptionalID{}, nil, asig, nil},
		{"ausente con cambio de asignatura lo limpia", OptionalID{}, actual, otraAsig, nil},
		{"null explícito lo quita", OptionalID{Set: true}, actual, asig, nil},
		{"un valor lo reemplaza", OptionalID{Set: true, Value: i64(52)}, actual, asig, i64(52)},
		{"un valor con cambio de asignatura", OptionalID{Set: true, Value: i64(60)}, actual, otraAsig, i64(60)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := temaTrasEditar(c.enviado, c.actual, asig, c.nueva); !sameID(got, c.want) {
				t.Errorf("got %v, want %v", deref(got), deref(c.want))
			}
		})
	}
}

func sameID(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func deref(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
