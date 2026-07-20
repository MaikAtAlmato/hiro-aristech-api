// internal/identity/last_name_aliases.go
package identity

import "strings"

// lastNameGroups lists known spelling variants of the same last name.
// Only groups where variants have different 3-rune prefixes are worth
// adding — identical prefixes are already caught by the main query.
var lastNameGroups = [][]string{
	{"Maier", "Mayer", "Meier", "Meyer"},
}

// lastNameAliases is the bidirectional lookup map built from lastNameGroups.
// Keys are lowercase (strings.ToLower).
var lastNameAliases map[string][]string

func init() {
	lastNameAliases = make(map[string][]string)
	for _, group := range lastNameGroups {
		for i, name := range group {
			key := strings.ToLower(name)
			for j, other := range group {
				if i != j {
					lastNameAliases[key] = append(lastNameAliases[key], other)
				}
			}
		}
	}
}

// lastNameAliasPrefixes returns the distinct 3-rune query prefixes for all
// known spelling variants of normalizedName, excluding the caller's own
// prefix. Returns nil when no aliases exist or all aliases share the caller's prefix.
func lastNameAliasPrefixes(normalizedName string) []string {
	aliases, ok := lastNameAliases[normalizedName]
	if !ok {
		return nil
	}
	ownPrefix := shortPrefix(normalizedName, 3)
	seen := map[string]bool{ownPrefix: true}
	var prefixes []string
	for _, alias := range aliases {
		aliasA, _ := normalize(alias)
		p := shortPrefix(aliasA, 3)
		if !seen[p] {
			seen[p] = true
			prefixes = append(prefixes, p)
		}
	}
	if len(prefixes) == 0 {
		return nil
	}
	return prefixes
}
