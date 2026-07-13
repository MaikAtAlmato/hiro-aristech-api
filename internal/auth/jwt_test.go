package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestTokenService_IssueAndVerify_RoundTrip(t *testing.T) {
	svc := NewTokenService("test-secret", 15*time.Minute)

	token, err := svc.Issue("Max Mustermann", "vm-1", "ms-1")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := svc.Verify(token)
	require.NoError(t, err)
	require.Equal(t, "Max Mustermann", claims.DisplayName)
	require.Equal(t, "vm-1", claims.ValuemationPersonID)
	require.Equal(t, "ms-1", claims.MsgraphPersonID)
}

func TestTokenService_Verify_ExpiredToken(t *testing.T) {
	svc := NewTokenService("test-secret", -1*time.Minute)

	token, err := svc.Issue("Max Mustermann", "vm-1", "ms-1")
	require.NoError(t, err)

	_, err = svc.Verify(token)
	require.Error(t, err)
}

func TestTokenService_Verify_WrongSecret(t *testing.T) {
	issuer := NewTokenService("secret-a", 15*time.Minute)
	verifier := NewTokenService("secret-b", 15*time.Minute)

	token, err := issuer.Issue("Max Mustermann", "vm-1", "ms-1")
	require.NoError(t, err)

	_, err = verifier.Verify(token)
	require.Error(t, err)
}

func TestTokenService_Verify_TamperedToken(t *testing.T) {
	svc := NewTokenService("test-secret", 15*time.Minute)

	token, err := svc.Issue("Max Mustermann", "vm-1", "ms-1")
	require.NoError(t, err)

	_, err = svc.Verify(token + "tampered")
	require.Error(t, err)
}

func TestTokenService_Verify_RejectsUnexpectedSigningMethod(t *testing.T) {
	svc := NewTokenService("test-secret", 15*time.Minute)

	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{}}
	unsignedToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := unsignedToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = svc.Verify(tokenString)
	require.Error(t, err)
}
