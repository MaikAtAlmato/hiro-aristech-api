// Package auth issues and verifies stateless JWTs for authenticated callers.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims are the JWT claims issued for an authenticated caller.
type Claims struct {
	jwt.RegisteredClaims
	ValuemationPersonID string `json:"valuemationPersonXid,omitempty"`
	MsgraphPersonID     string `json:"msgraphPersonXid,omitempty"`
	DisplayName         string `json:"displayName,omitempty"`
}

// TokenService issues and verifies HMAC-signed, stateless JWTs.
type TokenService struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenService creates a new TokenService with the given signing secret and token lifetime.
func NewTokenService(secret string, ttl time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), ttl: ttl}
}

// TTL returns the configured token lifetime.
func (s *TokenService) TTL() time.Duration {
	return s.ttl
}

// Issue signs a new token for the given resolved identity. An empty
// valuemationPersonXID or msgraphPersonXID means that source had no match.
func (s *TokenService) Issue(displayName, valuemationPersonXID, msgraphPersonXID string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
		ValuemationPersonID: valuemationPersonXID,
		MsgraphPersonID:     msgraphPersonXID,
		DisplayName:         displayName,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates tokenString, returning its claims if valid,
// unexpired, and signed with this service's secret using HMAC.
func (s *TokenService) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
