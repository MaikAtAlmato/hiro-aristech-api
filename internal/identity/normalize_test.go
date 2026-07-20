// internal/identity/normalize_test.go
package identity

import "testing"

import "github.com/stretchr/testify/require"

func TestNormalize_Umlauts(t *testing.T) {
	a, b := normalize("Döllinger")
	require.Equal(t, "doellinger", a)
	require.Equal(t, "dollinger", b)
}

func TestNormalize_SpecialCharactersAndWhitespace(t *testing.T) {
	a, b := normalize("Weiß-Müller")
	require.Equal(t, "weiss mueller", a)
	require.Equal(t, "weiss muller", b)
}

func TestNormalize_CollapsesMultipleSpaces(t *testing.T) {
	a, b := normalize("Max   Mustermann")
	require.Equal(t, "max mustermann", a)
	require.Equal(t, "max mustermann", b)
}

func TestNormalize_NoSpecialCharacters_BothVariantsIdentical(t *testing.T) {
	a, b := normalize("Schmidt")
	require.Equal(t, "schmidt", a)
	require.Equal(t, "schmidt", b)
}

func TestNormalize_TrimsWhitespace(t *testing.T) {
	a, b := normalize("  Meyer  ")
	require.Equal(t, "meyer", a)
	require.Equal(t, "meyer", b)
}
