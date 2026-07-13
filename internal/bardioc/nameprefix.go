// internal/bardioc/nameprefix.go
package bardioc

import "regexp"

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
