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
