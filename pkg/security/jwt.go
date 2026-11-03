// Package security provides pure cryptographic utilities.
// It does not manage sessions, tokens in storage, or any stateful resources.
package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims embeds standard registered JWT claims.
type Claims struct {
	jwt.RegisteredClaims
}

// TokenSigner signs a subject into a JWT string with a given TTL.
type TokenSigner interface {
	Sign(subject string, ttl time.Duration) (string, error)
}

// TokenParser parses and validates a JWT string, returning its Claims.
type TokenParser interface {
	Parse(tokenString string) (*Claims, error)
}

// TokenManager combines signing and parsing.
type TokenManager interface {
	TokenSigner
	TokenParser
}

type jwtHS256Manager struct{ signingKey []byte }

// NewJWTManager returns a TokenManager backed by HS256 with the given signing key.
func NewJWTManager(signingKey string) (TokenManager, error) {
	if signingKey == "" {
		return nil, errors.New("security: signing key must not be empty")
	}
	return &jwtHS256Manager{signingKey: []byte(signingKey)}, nil
}

func (m *jwtHS256Manager) Sign(subject string, ttl time.Duration) (string, error) {
	if subject == "" {
		return "", errors.New("security: empty subject")
	}
	if ttl <= 0 {
		return "", errors.New("security: ttl must be positive")
	}

	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(m.signingKey)
	if err != nil {
		return "", fmt.Errorf("security: sign token: %w", err)
	}
	return ss, nil
}

func (m *jwtHS256Manager) Parse(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("security: empty token string")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("security: unexpected signing method")
		}
		return m.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("security: parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("security: invalid token claims")
	}
	return claims, nil
}
