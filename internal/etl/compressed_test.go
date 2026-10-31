package etl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDetectCompressionRefusesBzip2(t *testing.T) {
	// The magic bytes of a bzip2 file.
	input := "BZh91AY&SY" + strings.Repeat("\x00", 32)

	_, err := DetectCompression(strings.NewReader(input))
	if !errors.Is(err, ErrCompressedInput) {
		t.Errorf("error = %v, want ErrCompressedInput", err)
	}
}

func TestDetectCompressionRefusesTheOtherFormats(t *testing.T) {
	cases := map[string]string{
		"gzip": "\x1f\x8b\x08\x00",
		"zstd": "\x28\xb5\x2f\xfd",
		"xz":   "\xfd7zXZ\x00",
		"zip":  "PK\x03\x04",
	}

	for name, magic := range cases {
		_, err := DetectCompression(strings.NewReader(magic + strings.Repeat("\x00", 16)))
		if !errors.Is(err, ErrCompressedInput) {
			t.Errorf("%s: error = %v, want ErrCompressedInput", name, err)
		}
	}
}

func TestDetectCompressionAcceptsAJsonStream(t *testing.T) {
	input := `{"id":"Q90","type":"item"}` + "\n" + `{"id":"Q1","type":"item"}`

	reader, err := DetectCompression(strings.NewReader(input))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// The stream is not consumed by the detection.
	content, err := readAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if content != input {
		t.Errorf("the stream was consumed:\n got: %q\nwant: %q", content, input)
	}
}

func TestDetectCompressionAcceptsAnEmptyStream(t *testing.T) {
	reader, err := DetectCompression(strings.NewReader(""))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	content, err := readAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if content != "" {
		t.Errorf("content = %q, want empty", content)
	}
}

func TestThePipelineRefusesACompressedStream(t *testing.T) {
	var items, properties bytes.Buffer

	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget(ParisQID, "", "", NewEntityWriter(&items), false),
		},
		Properties:      &properties,
		WriteProperties: true,
	})

	_, err := pipeline.Run(context.Background(), strings.NewReader("BZh91AY&SY\x00\x00"))
	if !errors.Is(err, ErrCompressedInput) {
		t.Errorf("error = %v, want ErrCompressedInput", err)
	}

	// Nothing was written: the run stopped before the first line.
	if items.Len() != 0 || properties.Len() != 0 {
		t.Errorf("the run wrote %d and %d bytes", items.Len(), properties.Len())
	}
}

// readAll reads a whole reader.
func readAll(r io.Reader) (string, error) {
	content, err := io.ReadAll(r)
	return string(content), err
}
