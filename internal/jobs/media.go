package jobs

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
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
// it stays correct no matter what the file is called. Storing a JPEG
// under a ".png" name produces a file that some tools reject and all of
// them misidentify.
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

// ImageSize describes the pixel dimensions of an image.
type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DimensionsOf returns the pixel dimensions of the given image bytes.
//
// It fails for data that is not a decodable image, which is the normal
// case for a non-image artifact.
func DimensionsOf(data []byte) (ImageSize, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageSize{}, fmt.Errorf("decode image: %w", err)
	}
	return ImageSize{Width: cfg.Width, Height: cfg.Height}, nil
}
