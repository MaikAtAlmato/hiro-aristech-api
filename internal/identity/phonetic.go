// internal/identity/phonetic.go
package identity

import "strings"

// koelnerPhonetik returns the standard Kölner Phonetik (Cologne phonetic,
// Postel 1969) code for name, capturing German pronunciation regardless of
// spelling variants — e.g. "meyer" and "maier" both encode to "67".
// Operates on the ASCII-stripped form of a name (see normalize's variant
// B); only A-Z letters contribute to the code, everything else is
// ignored.
func koelnerPhonetik(name string) string {
	letters := onlyLetters(strings.ToUpper(name))
	if len(letters) == 0 {
		return ""
	}

	var raw []byte
	for i, r := range letters {
		var prev, next rune
		if i > 0 {
			prev = letters[i-1]
		}
		if i < len(letters)-1 {
			next = letters[i+1]
		}
		raw = append(raw, letterCode(r, prev, next, i == 0)...)
	}

	// Collapse consecutive identical digits (e.g. the double "L" in
	// "sellner" produces two adjacent "5"s, collapsed to one — this is
	// what makes "selner" and "sellner" encode identically).
	collapsed := make([]byte, 0, len(raw))
	for i, d := range raw {
		if i == 0 || d != raw[i-1] {
			collapsed = append(collapsed, d)
		}
	}

	// Remove every '0' digit except a leading one.
	out := make([]byte, 0, len(collapsed))
	for i, d := range collapsed {
		if d == '0' && i != 0 {
			continue
		}
		out = append(out, d)
	}
	return string(out)
}

// onlyLetters returns s (expected already uppercase) with every rune
// outside A-Z removed.
func onlyLetters(s string) []rune {
	var out []rune
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			out = append(out, r)
		}
	}
	return out
}

// letterCode returns the Kölner Phonetik digit(s) for letter r, given its
// previous and next letters ('\x00' if there is none) and whether r is
// the first letter of the name. Returns "" for H, which contributes no
// digit. X is the only letter that can contribute two digits at once
// ("48", representing the "ks" sound).
func letterCode(r, prev, next rune, isFirst bool) string {
	isOneOf := func(c rune, options ...rune) bool {
		for _, o := range options {
			if c == o {
				return true
			}
		}
		return false
	}

	switch r {
	case 'A', 'E', 'I', 'J', 'O', 'U', 'Y':
		return "0"
	case 'H':
		return ""
	case 'B':
		return "1"
	case 'P':
		if next == 'H' {
			return "3"
		}
		return "1"
	case 'D', 'T':
		if isOneOf(next, 'C', 'S', 'Z') {
			return "8"
		}
		return "2"
	case 'F', 'V', 'W':
		return "3"
	case 'G', 'K', 'Q':
		return "4"
	case 'C':
		if isFirst {
			if isOneOf(next, 'A', 'H', 'K', 'L', 'O', 'Q', 'R', 'U', 'X') {
				return "4"
			}
			return "8"
		}
		if isOneOf(prev, 'S', 'Z') {
			return "8"
		}
		if isOneOf(next, 'A', 'H', 'K', 'O', 'Q', 'U', 'X') {
			return "4"
		}
		return "8"
	case 'X':
		if isOneOf(prev, 'C', 'K', 'Q') {
			return "8"
		}
		return "48"
	case 'L':
		return "5"
	case 'M', 'N':
		return "6"
	case 'R':
		return "7"
	case 'S', 'Z':
		return "8"
	default:
		return ""
	}
}
