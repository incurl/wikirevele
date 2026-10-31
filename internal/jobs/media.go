package jobs

import (
	"bytes"
)

// MediaType describes a detected media format.
type MediaType struct {
	// Extension is the file extension, without the dot.
	Extension string
	// Mime is the MIME type.
	Mime string
}

// DetectMediaType identifies the format of the given bytes.
//
// The format is read from the bytes rather than from the file name, so
// it stays correct no matter what the file is called. Storing a file
// under a name that does not match its content produces a file that
// some tools reject and all of them misidentify.
//
// The detection is about the bytes that were stored, not about what
// they mean: the queue does not know what kind of work produced them.
func DetectMediaType(data []byte) MediaType {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return MediaType{Extension: "jpg", Mime: "image/jpeg"}
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return MediaType{Extension: "png", Mime: "image/png"}
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return MediaType{Extension: "gif", Mime: "image/gif"}
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return MediaType{Extension: "webp", Mime: "image/webp"}
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("%PDF")):
		return MediaType{Extension: "pdf", Mime: "application/pdf"}
	default:
		// An unknown format is still stored, but as a PNG name, which
		// is the safest neutral fallback.
		return MediaType{Extension: "bin", Mime: "application/octet-stream"}
	}
}

// MimeTypeOf returns the MIME type of the given bytes.
func MimeTypeOf(data []byte) string {
	return DetectMediaType(data).Mime
}
