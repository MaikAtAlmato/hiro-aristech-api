// internal/identity/fuzzy.go
package identity

// minFuzzyNameLength is the shortest name part fuzzy matching will
// attempt to correct. Shorter parts (e.g. a single initial) already get
// full recall through the exact-prefix path, and edit-distance
// correction on a 1-2 character string is too permissive to trust — e.g.
// distance 1 from a single letter matches almost any other letter.
const minFuzzyNameLength = 3

// levenshtein returns the edit distance between a and b (insertions,
// deletions, substitutions), operating on runes so multi-byte characters
// (e.g. "ü") each count as one unit, not one per UTF-8 byte.
func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)

	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = minInt(del, minInt(ins, sub))
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fuzzyThreshold returns the maximum edit distance still considered a
// fuzzy match for a name of this length (rune count): round(len/4),
// rounded half up, floored at 1. Short names tolerate 1 edit; longer
// names tolerate proportionally more, so the tolerance scales with name
// length instead of using one fixed number that would be too loose for
// short names and too strict for long ones.
func fuzzyThreshold(name string) int {
	n := len([]rune(name))
	t := int(float64(n)/4.0 + 0.5)
	if t < 1 {
		return 1
	}
	return t
}

// shortPrefix returns the first n runes of s, or all of s if it has fewer
// than n runes. Rune-based (not byte-based) so multi-byte characters
// aren't split mid-character.
func shortPrefix(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
