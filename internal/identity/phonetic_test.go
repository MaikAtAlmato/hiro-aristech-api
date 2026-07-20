// internal/identity/phonetic_test.go
package identity

import "testing"

import "github.com/stretchr/testify/require"

func TestKoelnerPhonetik_MeyerAndMaier_SameCode(t *testing.T) {
	require.Equal(t, "67", koelnerPhonetik("meyer"))
	require.Equal(t, "67", koelnerPhonetik("maier"))
}

func TestKoelnerPhonetik_DoubledConsonant_CollapsesSameAsSingle(t *testing.T) {
	require.Equal(t, koelnerPhonetik("selner"), koelnerPhonetik("sellner"))
	require.Equal(t, "8567", koelnerPhonetik("selner"))
}

func TestKoelnerPhonetik_EmptyInput_ReturnsEmpty(t *testing.T) {
	require.Equal(t, "", koelnerPhonetik(""))
}

func TestKoelnerPhonetik_NonLetterCharactersIgnored(t *testing.T) {
	require.Equal(t, koelnerPhonetik("mueller"), koelnerPhonetik("mueller123"))
}
