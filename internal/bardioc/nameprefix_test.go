// internal/bardioc/nameprefix_test.go
package bardioc

import "testing"

import "github.com/stretchr/testify/require"

func TestNamePrefixPattern(t *testing.T) {
	// No leading "^": Elasticsearch/Lucene regexp queries are always
	// implicitly anchored to match the whole term, and don't support "^"/"$"
	// as anchor operators. A literal "^" here would require the indexed
	// name to actually start with a caret character, which never happens —
	// so a leading "^" makes every prefix query return zero rows.
	require.Equal(t, "M.*", namePrefixPattern("M"))
	require.Equal(t, "Maik.*", namePrefixPattern("Maik"))
	require.Equal(t, `M\..*`, namePrefixPattern("M."))
	require.Equal(t, `Mü.*`, namePrefixPattern("Mü"))
}

func TestLastNamePrefixVariants_SGroup(t *testing.T) {
	// S, Z, C are phonetically equivalent at word start (code 8)
	variants := lastNamePrefixVariants("Sel")
	require.ElementsMatch(t, []string{"Sel", "Zel", "Cel"}, variants)
}

func TestLastNamePrefixVariants_KGroup(t *testing.T) {
	// G, K, Q, C (k-sound) are phonetically equivalent at word start (code 4)
	variants := lastNamePrefixVariants("Gal")
	require.ElementsMatch(t, []string{"Gal", "Kal", "Qal", "Cal"}, variants)
}

func TestLastNamePrefixVariants_CGroup(t *testing.T) {
	// C expands to both G/K/Q (k-sound) AND S/Z (s-sound) groups
	variants := lastNamePrefixVariants("Car")
	require.ElementsMatch(t, []string{"Car", "Gar", "Kar", "Qar", "Sar", "Zar"}, variants)
}

func TestLastNamePrefixVariants_BPGroup(t *testing.T) {
	variants := lastNamePrefixVariants("Bau")
	require.ElementsMatch(t, []string{"Bau", "Pau"}, variants)
}

func TestLastNamePrefixVariants_NoEquivalents(t *testing.T) {
	// L has no equivalents
	variants := lastNamePrefixVariants("Lan")
	require.Equal(t, []string{"Lan"}, variants)
}

func TestLastNamePrefixVariants_EmptyPrefix(t *testing.T) {
	variants := lastNamePrefixVariants("")
	require.Equal(t, []string{""}, variants)
}

func TestLastNamePrefixVariants_SingleRune(t *testing.T) {
	variants := lastNamePrefixVariants("S")
	require.ElementsMatch(t, []string{"S", "Z", "C"}, variants)
}

func TestLastNamePrefixVariants_UppercasePreserved(t *testing.T) {
	variants := lastNamePrefixVariants("SELL")
	require.ElementsMatch(t, []string{"SELL", "ZELL", "CELL"}, variants)
}

func TestLastNamePrefixVariants_LowercasePreserved(t *testing.T) {
	variants := lastNamePrefixVariants("sell")
	require.ElementsMatch(t, []string{"sell", "zell", "cell"}, variants)
}
