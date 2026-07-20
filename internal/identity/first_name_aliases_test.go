package identity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstNameAliasPrefixes_Mike(t *testing.T) {
	// "mike" aliases "Maik" (mai) and "Marek" (mar); own prefix "mik"
	got := firstNameAliasPrefixes("mike")
	require.ElementsMatch(t, []string{"mai", "mar"}, got)
}

func TestFirstNameAliasPrefixes_Maik_Bidirectional(t *testing.T) {
	// "maik" aliases "Mike" (mik) and "Marek" (mar); own prefix "mai"
	got := firstNameAliasPrefixes("maik")
	require.ElementsMatch(t, []string{"mik", "mar"}, got)
}

func TestFirstNameAliasPrefixes_Philipp_SamePrefixDeduped(t *testing.T) {
	// "philipp", "philip", "phillip" all normalize to prefix "phi".
	// Own prefix is "phi"; aliases also produce "phi" → all deduped out.
	got := firstNameAliasPrefixes("philipp")
	require.Nil(t, got)
}

func TestFirstNameAliasPrefixes_NoAlias(t *testing.T) {
	// "emma" is not in any alias group
	got := firstNameAliasPrefixes("emma")
	require.Nil(t, got)
}

func TestFirstNameAliasPrefixes_Marc(t *testing.T) {
	// "marc" → alias "Mark" → normalize → "mark" → prefix "mar"
	// own prefix = "mar" — same, so deduped out → nil
	got := firstNameAliasPrefixes("marc")
	require.Nil(t, got)
}

func TestFirstNameAliasPrefixes_Stefan(t *testing.T) {
	// "stefan" → alias "Stephan" → normalize → "stephan" → prefix "ste"
	// own prefix = "ste" — same → nil
	got := firstNameAliasPrefixes("stefan")
	require.Nil(t, got)
}
