// internal/identity/fuzzy.go
package identity

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
