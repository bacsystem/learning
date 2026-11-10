// Package pgerr classifies PostgreSQL errors returned by lib/pq.
package pgerr

import (
	"errors"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

const (
	codeUniqueViolation = "23505"
	codeFKViolation     = "23503"
)

func hasCode(err error, code pqerror.Code) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == code
}

// IsUniqueViolation reports a unique_violation (23505), also when wrapped.
func IsUniqueViolation(err error) bool { return hasCode(err, codeUniqueViolation) }

// IsFKViolation reports a foreign_key_violation (23503), also when wrapped.
func IsFKViolation(err error) bool { return hasCode(err, codeFKViolation) }
