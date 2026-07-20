// internal/identity/normalize.go
package identity

import (
	"regexp"
	"strings"
)

var nonLetterRe = regexp.MustCompile(`[^a-zA-Z]+`)

// normalize returns two normalized forms of a name part for matching:
// variant A (German transliteration: ä→ae, ö→oe, ü→ue, ß→ss) and variant B
// (ASCII-stripped: ä→a, ö→o, ü→u, ß→ss). Both variants are additionally
// lowercased, with non-letter characters (hyphens, apostrophes, etc.)
// collapsed to a single space and trimmed.
func normalize(name string) (variantA, variantB string) {
	lower := strings.ToLower(strings.TrimSpace(name))

	a := transliterate(lower, true)
	b := transliterate(lower, false)

	return cleanupSpacing(a), cleanupSpacing(b)
}

// transliterate replaces German umlauts and ß. If keepE is true, umlauts
// expand to their two-letter German transliteration (ä→ae); otherwise they
// collapse to the plain base letter (ä→a). ß always becomes "ss".
func transliterate(s string, keepE bool) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case 'ä':
			if keepE {
				sb.WriteString("ae")
			} else {
				sb.WriteRune('a')
			}
		case 'ö':
			if keepE {
				sb.WriteString("oe")
			} else {
				sb.WriteRune('o')
			}
		case 'ü':
			if keepE {
				sb.WriteString("ue")
			} else {
				sb.WriteRune('u')
			}
		case 'ß':
			sb.WriteString("ss")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// cleanupSpacing collapses runs of non-letter characters into a single
// space and trims the result.
func cleanupSpacing(s string) string {
	return strings.TrimSpace(nonLetterRe.ReplaceAllString(s, " "))
}
