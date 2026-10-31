package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestStripArrayIfNeededReadsAnArray(t *testing.T) {
	input := "[\n{\"a\":1},\n{\"a\":2}\n]\n"

	reader, isArray := StripArrayIfNeeded(strings.NewReader(input))
	if !isArray {
		t.Fatal("the stream is an array, it was not detected")
	}

	content, err := readAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// The bracket is gone, and the rest is untouched.
	want := "{\"a\":1},\n{\"a\":2}\n]\n"
	if content != want {
		t.Errorf("content:\n got: %q\nwant: %q", content, want)
	}
}

func TestStripArrayIfNeededLeavesJsonLinesAlone(t *testing.T) {
	input := "{\"a\":1}\n{\"a\":2}\n"

	reader, isArray := StripArrayIfNeeded(strings.NewReader(input))
	if isArray {
		t.Fatal("the stream is JSON Lines, it was taken for an array")
	}

	content, err := readAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if content != input {
		t.Errorf("the first line was lost:\n got: %q\nwant: %q", content, input)
	}
}

func TestStripArrayIfNeededKeepsTheLongFirstEntity(t *testing.T) {
	// The first entity is longer than the buffer of the detection. It
	// must come out whole, and not cut at the end of the head.
	long := strings.Repeat("x", 100_000)
	input := "[\n{\"a\":\"" + long + "\"},\n{\"a\":2}\n]\n"

	reader, isArray := StripArrayIfNeeded(strings.NewReader(input))
	if !isArray {
		t.Fatal("the stream is an array, it was not detected")
	}

	content, err := readAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	first := strings.Split(content, "\n")[0]
	if !strings.Contains(first, long) {
		t.Errorf("the first entity was cut: it is %d bytes, want %d", len(first), len(long)+11)
	}
}

func TestStripArrayIfNeededAcceptsAnEmptyStream(t *testing.T) {
	reader, isArray := StripArrayIfNeeded(strings.NewReader(""))
	if isArray {
		t.Error("an empty stream is not an array")
	}

	content, err := readAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if content != "" {
		t.Errorf("content = %q, want empty", content)
	}
}

func TestTrimEntityComma(t *testing.T) {
	cases := map[string]string{
		`{"a":1},`:  `{"a":1}`,   // the comma of an array element
		`{"a":1}`:   `{"a":1}`,   // the last element
		`{"a":1}, `: `{"a":1}`,   // a comma and a space
		`{"a":","}`: `{"a":","}`, // a comma inside a string
	}

	for input, want := range cases {
		if got := string(TrimEntityComma([]byte(input))); got != want {
			t.Errorf("TrimEntityComma(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsArrayClose(t *testing.T) {
	if !IsArrayClose([]byte("]")) {
		t.Error("the bracket is the close of the array")
	}
	if !IsArrayClose([]byte(" ] \r")) {
		t.Error("the bracket with whitespace is the close of the array")
	}
	if IsArrayClose([]byte(`{"a":1}`)) {
		t.Error("an entity is not the close of the array")
	}
}

func TestThePipelineReadsAnArray(t *testing.T) {
	// The same entities, written as a JSON array, are read as if the
	// dump were JSON Lines.
	entity := func(id string) string {
		return `{"id":"` + id + `","type":"item",` +
			`"labels":{"en":{"language":"en","value":"` + id + `"}},` +
			`"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131",` +
			`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},` +
			`"type":"wikibase-entityid"}}}]}}`
	}

	input := "[\n" + entity("Q1") + ",\n" + entity("Q2") + "\n]\n"

	var items, properties bytes.Buffer
	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget(ParisQID, "", "", NewEntityWriter(&items), false),
		},
		Properties:      &properties,
		WriteProperties: true,
	})

	stats, err := pipeline.Run(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if stats.Malformed != 0 {
		t.Errorf("malformed = %d, want 0", stats.Malformed)
	}
	if stats.Items != 2 {
		t.Errorf("items = %d, want 2", stats.Items)
	}
	// The closing bracket is not an entity, and it is not counted.
	if stats.Scanned != 2 {
		t.Errorf("scanned = %d, want 2", stats.Scanned)
	}

	// Every line of the output is one entity.
	lines := strings.Split(strings.TrimSpace(items.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("the output has %d lines, want 2", len(lines))
	}
	for _, line := range lines {
		var out Entity
		if err := json.Unmarshal([]byte(line), &out); err != nil {
			t.Errorf("the output line is not an entity: %v", err)
		}
	}
}
