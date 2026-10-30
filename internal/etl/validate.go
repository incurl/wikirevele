package etl

import "strings"

// IsQID tells whether a string is a Wikidata item identifier.
//
// An identifier is a Q followed by one or more digits: Q90 is Paris,
// Q148 is Shanghai. The check is deliberately strict, because a target
// that is not an identifier matches nothing, and a run that matches
// nothing looks like a broken dump.
//
// The prefix is followed by digits only. Wikidata has no other kind of
// item identifier, and a property identifier (P131) is not an item.
func IsQID(value string) bool {
	if len(value) < 2 {
		return false
	}
	if value[0] != 'Q' {
		return false
	}

	return allDigits(value[1:])
}

// IsPID tells whether a string is a Wikidata property identifier.
//
// A property identifier is a P followed by one or more digits: P131 is
// "located in the administrative territorial entity".
func IsPID(value string) bool {
	if len(value) < 2 {
		return false
	}
	if value[0] != 'P' {
		return false
	}

	return allDigits(value[1:])
}

// IsLanguageCode tells whether a string is a language code.
//
// A code is a lower case letter, or a lower case letter and a
// hyphenated part: "en", "fr", "zh", "zh-hans", "be-tarask".
//
// The check is the shape of the code, not the shape of a real
// language: the dump holds codes that the standards do not list, and
// refusing them would refuse real data.
func IsLanguageCode(value string) bool {
	if value == "" {
		return false
	}

	// The code is at most twelve characters: the longest real one is
	// "zh-min-nan" and the region part is short.
	if len(value) > 12 {
		return false
	}

	parts := strings.Split(value, "-")
	if len(parts) > 3 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
		for i := 0; i < len(part); i++ {
			c := part[i]
			if c >= 'a' && c <= 'z' {
				continue
			}
			if c >= '0' && c <= '9' {
				continue
			}
			return false
		}
	}

	// The first part is the language, and it is letters only.
	for i := 0; i < len(parts[0]); i++ {
		c := parts[0][i]
		if c < 'a' || c > 'z' {
			return false
		}
	}

	return true
}

// allDigits tells whether a string is made of digits only.
//
// An empty string is not: an identifier has at least one digit.
func allDigits(value string) bool {
	if value == "" {
		return false
	}

	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}

	return true
}
