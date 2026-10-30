package etl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// The magic bytes of the compression formats a dump may arrive in.
//
// The command reads a decompressed stream, and it is not able to
// decompress one. A compressed stream is detected by its first bytes,
// so that the run stops at once with an explanation instead of
// reporting every line as malformed.
var compressedMagics = [][]byte{
	{'B', 'Z', 'h'},                  // bzip2
	{0x1f, 0x8b},                     // gzip
	{0x28, 0xb5, 0x2f, 0xfd},         // zstd
	{0xfd, '7', 'z', 'X', 'Z', 0x00}, // xz
	{'P', 'K', 0x03, 0x04},           // zip
}

// ErrCompressedInput is returned when the input is a compressed stream.
//
// The command does not decompress: the decompressor is another
// process, and the dump is piped in.
var ErrCompressedInput = fmt.Errorf(
	"the input is compressed: pipe it through a decompressor, " +
		"for example \"bzcat latest-all.json.bz2 | wikirevele-etl\"")

// DetectCompression tells whether a stream is compressed.
//
// It reads the first bytes of the stream and looks them up in the
// table of the known formats. The bytes are put back, so that the
// caller reads the whole stream.
func DetectCompression(r io.Reader) (io.Reader, error) {
	buffered := bufio.NewReaderSize(r, 4096)

	// The stream is too short to hold a magic: it is not compressed.
	head, err := buffered.Peek(6)
	if err != nil && len(head) == 0 {
		// The stream is empty: there is nothing to detect.
		return buffered, nil
	}

	for _, magic := range compressedMagics {
		if bytes.HasPrefix(head, magic) {
			return buffered, ErrCompressedInput
		}
	}

	return buffered, nil
}
