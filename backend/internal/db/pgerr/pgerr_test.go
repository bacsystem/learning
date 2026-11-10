package pgerr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
)

func TestClassify(t *testing.T) {
	uniq := &pq.Error{Code: "23505"}
	fk := &pq.Error{Code: "23503"}
	other := &pq.Error{Code: "23502"}
	cases := []struct {
		err        error
		uniq, isFK bool
	}{
		{uniq, true, false},
		{fmt.Errorf("wrapped: %w", uniq), true, false},
		{fk, false, true},
		{fmt.Errorf("wrapped: %w", fk), false, true},
		{other, false, false},
		{errors.New("23505"), false, false},
		{nil, false, false},
	}
	for i, c := range cases {
		if got := IsUniqueViolation(c.err); got != c.uniq {
			t.Errorf("%d: IsUniqueViolation(%v) = %v", i, c.err, got)
		}
		if got := IsFKViolation(c.err); got != c.isFK {
			t.Errorf("%d: IsFKViolation(%v) = %v", i, c.err, got)
		}
	}
}
