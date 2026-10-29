package jobs

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestDetectMediaType verifies that the format is read from the bytes
// and not from the file name.
func TestDetectMediaType(t *testing.T) {
	cases := []struct {
		name      string
		data      []byte
		extension string
		mime      string
	}{
		{
			name:      "jpeg",
			data:      []byte{0xFF, 0xD8, 0xFF, 0xE0},
			extension: "jpg",
			mime:      "image/jpeg",
		},
		{
			name:      "png",
			data:      []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
			extension: "png",
			mime:      "image/png",
		},
		{
			name:      "gif",
			data:      []byte("GIF89a"),
			extension: "gif",
			mime:      "image/gif",
		},
		{
			name:      "webp",
			data:      []byte("RIFF0000WEBP"),
			extension: "webp",
			mime:      "image/webp",
		},
		{
			name:      "pdf",
			data:      []byte("%PDF-1.7"),
			extension: "pdf",
			mime:      "application/pdf",
		},
		{
			name:      "unknown",
			data:      []byte("not a known format"),
			extension: "bin",
			mime:      "application/octet-stream",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			media := DetectMediaType(tc.data)
			if media.Extension != tc.extension {
				t.Errorf("extension = %q, want %q", media.Extension, tc.extension)
			}
			if media.Mime != tc.mime {
				t.Errorf("mime = %q, want %q", media.Mime, tc.mime)
			}
		})
	}
}

// TestDefaultTitle verifies that a title is derived from the prompt
// when the application does not supply one.
func TestDefaultTitle(t *testing.T) {
	cases := []struct {
		prompt string
		want   string
	}{
		{prompt: "a red bicycle", want: "a red bicycle"},
		{prompt: "first line\nsecond line", want: "first line"},
		{prompt: "  padded  ", want: "padded"},
		{prompt: "", want: "Job"},
	}

	for _, tc := range cases {
		if got := defaultTitle(tc.prompt); got != tc.want {
			t.Errorf("defaultTitle(%q) = %q, want %q", tc.prompt, got, tc.want)
		}
	}
}

// TestDefaultTitleTruncatesWithoutSplittingARune verifies that a long
// prompt is cut without splitting a UTF-8 sequence.
func TestDefaultTitleTruncatesWithoutSplittingARune(t *testing.T) {
	// Each rune is two bytes, so a naive byte cut lands inside one.
	prompt := ""
	for i := 0; i < 40; i++ {
		prompt += "é"
	}

	title := defaultTitle(prompt)

	// The truncated text, without the ellipsis, must still be valid
	// UTF-8: a cut in the middle of a rune would make it invalid.
	body := strings.TrimSuffix(title, "…")
	if !utf8.ValidString(body) {
		t.Errorf("the title was cut in the middle of a rune: %q", title)
	}
	if !strings.HasSuffix(title, "…") {
		t.Errorf("the title was not truncated: %q", title)
	}

	// The visible text must hold no more than the allowed number of
	// bytes.
	if len(body) > 60 {
		t.Errorf("the title body is %d bytes long, want at most 60", len(body))
	}
}
