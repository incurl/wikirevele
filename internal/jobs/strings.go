package jobs

import (
	"strings"
	"unicode/utf8"
)

// indexNewline returns the byte index of the first line break, or -1.
//
// Both the Unix and the Windows line endings are recognised.
func indexNewline(s string) int {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return idx
	}
	return strings.IndexByte(s, '\r')
}

// trimSpace removes the leading and the trailing whitespace.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// isRuneStart reports whether the byte at index i starts a UTF-8
// sequence.
//
// It is used to cut a string without splitting a rune in half.
func isRuneStart(b byte) bool {
	return utf8.RuneStart(b)
}
