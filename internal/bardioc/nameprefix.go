// internal/bardioc/nameprefix.go
package bardioc

import (
	"regexp"
	"unicode"
)

// namePrefixPattern builds a case-insensitive "starts with" regex pattern
// for an Elasticsearch query from a plain-text name prefix, which may be a
// full name or a single initial (e.g. from FindByNamePrefix).
//
// No leading "^": Elasticsearch/Lucene regexp queries are always implicitly
// anchored to match the whole indexed term, and "^"/"$" are not supported as
// anchor operators there. A leading "^" is sent through as a literal
// character, so the query would require the name to actually start with a
// caret — matching nothing — which silently made every prefix lookup
// (FindByNamePrefix, used by /auth/match) return zero results in production.
func namePrefixPattern(prefix string) string {
	return regexp.QuoteMeta(prefix) + ".*"
}

// lastNameSuffixPattern returns a regex that matches any term whose suffix
// (all chars after the first) equals the given suffix. The leading "."
// matches exactly one arbitrary character, so ".ellner.*" matches "Sellner",
// "Kellner", etc. Used when the STT system mis-recognised only the first
// letter of the last name.
func lastNameSuffixPattern(suffix string) string {
	return "." + regexp.QuoteMeta(suffix) + ".*"
}

// firstLetterGroups maps each letter (uppercase) to all letters that share
// its Kölner Phonetik code at first-word position. Only groups with more
// than one member are listed; letters absent from the map have no
// phonetically equivalent alternatives.
//
// C is context-dependent in Kölner Phonetik: before A/H/K/L/O/Q/R/U/X it
// maps to code 4 (K-sound, like Carl/Karl), otherwise to code 8 (S-sound).
// Since we only see the first letter without its successor, C is placed in
// both the G/K/Q group and the S/Z group so that "Carl"↔"Karl" and
// "Sel"↔"Cel" are both reachable in either direction.
var firstLetterGroups = map[rune][]rune{
	'B': {'B', 'P'},
	'P': {'B', 'P'},
	'D': {'D', 'T'},
	'T': {'D', 'T'},
	'F': {'F', 'V', 'W'},
	'V': {'F', 'V', 'W'},
	'W': {'F', 'V', 'W'},
	'C': {'C', 'G', 'K', 'Q', 'S', 'Z'},
	'G': {'C', 'G', 'K', 'Q'},
	'K': {'C', 'G', 'K', 'Q'},
	'Q': {'C', 'G', 'K', 'Q'},
	'M': {'M', 'N'},
	'N': {'M', 'N'},
	'S': {'C', 'S', 'Z'},
	'Z': {'C', 'S', 'Z'},
}

// lastNamePrefixVariants returns all phonetically equivalent first-letter
// variants of prefix. The original prefix is always included. For an empty
// prefix or a first letter with no equivalents, a single-element slice with
// the original prefix is returned.
//
// Only the first rune is substituted; the rest of the prefix is kept as-is.
// Case of the substituted letter follows the case of the original first rune.
func lastNamePrefixVariants(prefix string) []string {
	runes := []rune(prefix)
	if len(runes) == 0 {
		return []string{prefix}
	}

	first := runes[0]
	group, ok := firstLetterGroups[unicode.ToUpper(first)]
	if !ok {
		return []string{prefix}
	}

	isUpper := unicode.IsUpper(first)
	seen := map[string]bool{}
	var out []string
	for _, alt := range group {
		if !isUpper {
			alt = unicode.ToLower(alt)
		}
		variant := string(alt) + string(runes[1:])
		if !seen[variant] {
			seen[variant] = true
			out = append(out, variant)
		}
	}
	return out
}
