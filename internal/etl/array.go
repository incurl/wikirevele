package etl

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// The bytes that frame a dump written as a JSON array.
//
// The official dump is JSON Lines: one entity per line. Some mirrors
// publish the same entities as one JSON array, which is a single JSON
// value spread over many lines:
//
//	[
//	{"type":"item","id":"Q31",...},
//	{"type":"item","id":"Q42",...}
//	]
//
// The framing is removed from the stream, so that the scanner sees the
// same lines in both forms: the entities, one per line, without the
// commas that separated them in the array.
var (
	// arrayOpen is the opening bracket, on a line of its own.
	arrayOpen = []byte("[")
	// arrayClose is the closing bracket, on a line of its own.
	arrayClose = []byte("]")
)

// StripArrayFraming removes the opening bracket of a dump written as
// a JSON array.
//
// The bracket is on a line of its own. The bytes of the head that come
// after it are kept, and the bytes that come before it are dropped.
//
// The head is returned whole, apart from the bracket: the entity that
// the head cuts in the middle of is still in the reader that follows,
// and the two have to be joined again.
func StripArrayFraming(head []byte) []byte {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if !bytes.HasPrefix(trimmed, arrayOpen) {
		return head
	}

	// The bracket is one byte, and the whitespace before it is kept
	// out of the way.
	offset := len(head) - len(trimmed) + len(arrayOpen)

	return head[offset:]
}

// IsArrayClose tells whether a line is the closing bracket of a JSON
// array.
//
// The line is the closing bracket and nothing else: the array is
// written one entity per line, so the bracket is on a line of its own.
func IsArrayClose(line []byte) bool {
	return bytes.Equal(bytes.TrimSpace(line), arrayClose)
}

// TrimEntityComma removes the comma that separated two entities of a
// JSON array.
//
// The comma is the last byte of the line, before the newline.
func TrimEntityComma(line []byte) []byte {
	trimmed := bytes.TrimRight(line, " \t\r")

	if len(trimmed) > 0 && trimmed[len(trimmed)-1] == ',' {
		return trimmed[:len(trimmed)-1]
	}

	return trimmed
}

// StripArrayIfNeeded removes the framing of a JSON array, if the stream
// is one.
//
// The opening bracket is on a line of its own, so the first line is
// read and looked at. The line is put back when it is not the bracket,
// so that the caller reads the whole stream.
//
// The second result tells whether the stream is an array: when it is,
// the commas and the closing bracket have to be removed from the lines.
func StripArrayIfNeeded(r io.Reader) (io.Reader, bool) {
	buffered := bufio.NewReaderSize(r, 4096)

	// The first line is read with the reader, not with a peek: a peek
	// reads a fixed number of bytes, and it would cut the first entity
	// in the middle.
	first, err := buffered.ReadString('\n')
	if err != nil && first == "" {
		// The stream is empty: there is nothing to detect.
		return buffered, false
	}

	if !bytes.Equal(bytes.TrimSpace([]byte(first)), arrayOpen) {
		// The first line is not the bracket: it is the first entity,
		// and the stream is JSON Lines.
		return io.MultiReader(strings.NewReader(first), buffered), false
	}

	// The bracket is dropped, and the entities follow.
	return buffered, true
}
