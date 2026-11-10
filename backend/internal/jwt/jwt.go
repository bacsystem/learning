package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenDuration = 24 * time.Hour

type Claims struct {
	UserID         int64  `json:"user_id"`
	Rol            string `json:"rol"`
	GradoID        int64  `json:"grado_id"`
	Nivel          string `json:"nivel"`
	EspecialidadID int64  `json:"especialidad_id"`
	InstitucionID  int64  `json:"institucion_id"`
	jwt.RegisteredClaims
}

// Issue creates a signed HS256 JWT token.
func Issue(secret string, userID int64, rol, nivel string, gradoID, especialidadID, institucionID int64) (string, int64, error) {
	now := time.Now()
	exp := now.Add(tokenDuration)
	claims := Claims{
		UserID:         userID,
		Rol:            rol,
		GradoID:        gradoID,
		Nivel:          nivel,
		EspecialidadID: especialidadID,
		InstitucionID:  institucionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, exp.Unix(), err
}

// Validate parses and validates a signed JWT, returning its claims. The exp
// claim is required (a token without it never expires otherwise).
func Validate(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
