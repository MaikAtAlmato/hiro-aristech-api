// internal/identity/fuzzy_test.go
package identity

import "testing"

import "github.com/stretchr/testify/require"

func TestLevenshtein(t *testing.T) {
	require.Equal(t, 0, levenshtein("Sellner", "Sellner"))
	require.Equal(t, 1, levenshtein("Selner", "Sellner")) // reported real case: missing "l"
	require.Equal(t, 3, levenshtein("kitten", "sitting")) // classic reference case: k→s, e→i, insert g
	require.Equal(t, 0, levenshtein("", ""))
	require.Equal(t, 3, levenshtein("", "abc"))
	require.Equal(t, 1, levenshtein("Mü", "Mu")) // multi-byte rune substitution counts as 1, not 2
}

func TestFuzzyThreshold(t *testing.T) {
	require.Equal(t, 1, fuzzyThreshold("a"))          // 1 rune: round(0.25)=0 -> floored to 1
	require.Equal(t, 1, fuzzyThreshold("abc"))        // 3 runes: round(0.75)=1
	require.Equal(t, 1, fuzzyThreshold("abcde"))      // 5 runes: round(1.25)=1
	require.Equal(t, 2, fuzzyThreshold("abcdef"))     // 6 runes: round(1.5)=2
	require.Equal(t, 2, fuzzyThreshold("abcdefghi"))  // 9 runes: round(2.25)=2
	require.Equal(t, 3, fuzzyThreshold("abcdefghij")) // 10 runes: round(2.5)=3
}

func TestShortPrefix(t *testing.T) {
	require.Equal(t, "Sel", shortPrefix("Selner", 3))
	require.Equal(t, "Mü", shortPrefix("Mü", 3))     // shorter than n: returned as-is
	require.Equal(t, "Mü1", shortPrefix("Mü123", 3)) // multi-byte rune counted as ONE unit
}
