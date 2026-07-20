// internal/identity/first_name_aliases.go
package identity

import "strings"

// firstNameGroups lists known spelling variants of the same first name.
// Each inner slice is one equivalence group. This is the only place to
// add or remove aliases — firstNameAliases is derived from this slice.
var firstNameGroups = [][]string{
	{"Mike", "Maik", "Marek"},
	{"Marc", "Mark"},
	{"Stefan", "Stephan"},
	{"Stefanie", "Stephanie"},
	{"Christina", "Kristina"},
	{"Phillip", "Philip", "Philipp"},
	{"Kai", "Kay"},
	{"Jan", "Yan"},
	{"Nico", "Niko"},
	{"Lukas", "Lucas", "Luca"},
	{"Sara", "Sarah"},
	{"Thomas", "Tomas"},
	{"Alexander", "Aleksander"},
	{"Kathrin", "Katrin"},
	{"Georg", "George"},
	{"Henrik", "Hendrik"},
	{"Nicole", "Nikole"},
	{"Simone", "Simona"},
	{"Veronika", "Veronica"},
	{"Mathias", "Matthias"},
	{"Markus", "Marcus"},
	{"Monika", "Monica"},
	{"Christian", "Kristian"},
	{"Michael", "Mikael", "Mikhail"},
}

// firstNameAliases is the bidirectional lookup map built from firstNameGroups.
// Keys are lowercase (strings.ToLower). Values are the other group members
// in their original casing (normalize() handles them at query time).
var firstNameAliases map[string][]string

func init() {
	firstNameAliases = make(map[string][]string)
	for _, group := range firstNameGroups {
		for i, name := range group {
			key := strings.ToLower(name)
			for j, other := range group {
				if i != j {
					firstNameAliases[key] = append(firstNameAliases[key], other)
				}
			}
		}
	}
}

// firstNameAliasPrefixes returns the distinct 3-rune query prefixes for all
// known spelling variants of normalizedName, excluding the caller's own
// prefix (which is already queried by the main prefix query). Returns nil
// when no aliases exist or all aliases share the caller's prefix.
//
// normalizedName must already be in the form returned by normalize() —
// lowercase, with umlauts transliterated.
func firstNameAliasPrefixes(normalizedName string) []string {
	aliases, ok := firstNameAliases[normalizedName]
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
