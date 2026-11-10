package examen

import (
	"errors"
	"testing"

	"github.com/cbaciliod/learning/internal/grading"
	"github.com/lib/pq"
)

func TestLetra(t *testing.T) {
	cases := map[int]string{1: "a", 2: "b", 26: "z", 0: "?", 27: "?", -1: "?"}
	for in, want := range cases {
		if got := Letra(in); got != want {
			t.Errorf("Letra(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPreguntaIDs(t *testing.T) {
	got := PreguntaIDs([]grading.Pregunta{{PreguntaID: 3}, {PreguntaID: 1}})
	if len(got) != 2 || got[0] != 3 || got[1] != 1 {
		t.Fatalf("PreguntaIDs = %v", got)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	err := &pq.Error{Code: "23505", Constraint: "uq_x"}
	if !IsUniqueViolation(err, "uq_x") {
		t.Fatal("same constraint not detected")
	}
	if IsUniqueViolation(err, "uq_y") || IsUniqueViolation(errors.New("x"), "uq_x") {
		t.Fatal("false positive")
	}
}
