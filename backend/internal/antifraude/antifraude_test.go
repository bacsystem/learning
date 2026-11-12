package antifraude

import (
	"bytes"
	"strings"
	"testing"
)

func TestClave(t *testing.T) {
	k1, k2 := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	tests := []struct {
		name         string
		claves       *Claves
		prefijo      string
		id           int64
		wantAnterior bool
	}{
		{"without rotation", New(k1, nil), PrefijoCuotaIA, 7, false},
		{"empty previous key is no rotation", New(k1, []byte{}), PrefijoCuotaIA, 7, false},
		{"during a rotation", New(k1, k2), PrefijoCuotaIA, 7, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, a := tt.claves.Clave(tt.prefijo, tt.id)
			if !strings.HasPrefix(v, tt.prefijo+":") || len(v) != len(tt.prefijo)+1+64 {
				t.Errorf("current key %q is not prefix:hex64", v)
			}
			if (a != "") != tt.wantAnterior {
				t.Errorf("previous key %q, want present = %v", a, tt.wantAnterior)
			}
			if a != "" && a == v {
				t.Error("previous and current keys are equal")
			}
		})
	}
}

func TestClave_Propiedades(t *testing.T) {
	c := New(bytes.Repeat([]byte{1}, 32), nil)
	a1, _ := c.Clave(PrefijoCuotaIA, 1)
	a1bis, _ := c.Clave(PrefijoCuotaIA, 1)
	a2, _ := c.Clave(PrefijoCuotaIA, 2)
	otro, _ := c.Clave("diag_cooldown", 1)
	if a1 != a1bis {
		t.Error("the derivation is not deterministic")
	}
	if a1 == a2 {
		t.Error("two users share a key")
	}
	if strings.SplitN(a1, ":", 2)[1] == strings.SplitN(otro, ":", 2)[1] {
		t.Error("the same user has the same HMAC under two prefixes: the domains are not separated")
	}
	nueva, _ := New(bytes.Repeat([]byte{9}, 32), nil).Clave(PrefijoCuotaIA, 1)
	if nueva == a1 {
		t.Error("a different secret gives the same key")
	}
}
