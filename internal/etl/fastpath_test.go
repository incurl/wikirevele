package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLooksLikeObject(t *testing.T) {
	cases := map[string]bool{
		`{"a":1}`:     true,
		`  {"a":1}  `: true,
		`[`:           false,
		`not json`:    false,
		`{"a":1`:      false, // truncated
		`"a"`:         false,
		``:            false,
	}

	for input, want := range cases {
		if got := looksLikeObject([]byte(input)); got != want {
			t.Errorf("looksLikeObject(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestTheFastPathSkipsALineThatCannotMatch(t *testing.T) {
	// The item mentions neither the target nor a property declaration,
	// so it is passed over without being decoded.
	line := `{"id":"Q1","type":"item","labels":{"en":{"language":"en","value":"x"}},` +
		`"claims":{"P31":[{"mainsnak":{"snaktype":"value","property":"P31",` +
		`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q5"},` +
		`"type":"wikibase-entityid"}}}]}}`

	stats, items := runOne(t, line)

	if stats.Items != 0 {
		t.Errorf("items = %d, want 0", stats.Items)
	}
	if stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", stats.Skipped)
	}
	if stats.Malformed != 0 {
		t.Errorf("malformed = %d, want 0", stats.Malformed)
	}
	if items != "" {
		t.Errorf("the item was written: %q", items)
	}
}

func TestTheFastPathKeepsAnItemThatMentionsTheTarget(t *testing.T) {
	line := `{"id":"Q1","type":"item",` +
		`"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131",` +
		`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},` +
		`"type":"wikibase-entityid"}}}]}}`

	stats, items := runOne(t, line)

	if stats.Items != 1 {
		t.Errorf("items = %d, want 1", stats.Items)
	}
	if !strings.Contains(items, `"Q1"`) {
		t.Errorf("the item was not written: %q", items)
	}
}

func TestTheFastPathKeepsAProperty(t *testing.T) {
	// A property never mentions the target, and it is kept anyway.
	line := `{"id":"P999","type":"property",` +
		`"labels":{"en":{"language":"en","value":"a property"}},"claims":{}}`

	stats, _ := runOne(t, line)

	// The item writer is what runOne inspects, so the property is
	// checked through the statistics.
	if stats.Properties != 1 {
		t.Errorf("properties = %d, want 1", stats.Properties)
	}
}

func TestTheFastPathStillReportsDamage(t *testing.T) {
	// A damaged line is reported even when it could not have matched:
	// a dump that arrived truncated has to be seen.
	stats, _ := runOne(t, `not json at all`)

	if stats.Malformed != 1 {
		t.Errorf("malformed = %d, want 1", stats.Malformed)
	}
}

func TestTheCountersReconcile(t *testing.T) {
	match := `{"id":"Q1","type":"item",` +
		`"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131",` +
		`"datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},` +
		`"type":"wikibase-entityid"}}}]}}`
	noMatch := `{"id":"Q2","type":"item","claims":{}}`
	property := `{"id":"P1","type":"property","claims":{}}`
	damaged := `{broken`

	input := strings.Join([]string{match, noMatch, property, damaged}, "\n")

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

	total := stats.Items + stats.Properties + stats.Skipped + stats.Malformed
	if total != stats.Scanned {
		t.Errorf("%d + %d + %d + %d = %d, want scanned = %d",
			stats.Items, stats.Properties, stats.Skipped, stats.Malformed,
			total, stats.Scanned)
	}
}

// runOne runs the pipeline over one line, and returns the statistics
// and the items that were written.
func runOne(t *testing.T, line string) (Stats, string) {
	t.Helper()

	var items, properties bytes.Buffer
	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget(ParisQID, "", "", NewEntityWriter(&items), false),
		},
		Properties:      &properties,
		WriteProperties: true,
	})

	stats, err := pipeline.Run(context.Background(), strings.NewReader(line))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// The output is checked to be valid NDJSON.
	for _, out := range strings.Split(strings.TrimSpace(items.String()), "\n") {
		if out == "" {
			continue
		}
		var entity Entity
		if err := json.Unmarshal([]byte(out), &entity); err != nil {
			t.Fatalf("the output is not an entity: %v", err)
		}
	}

	return stats, items.String()
}

func TestTheFastPathFindsAPropertyWithSpacesAroundTheColon(t *testing.T) {
	// A mirror that writes the JSON with spaces around the colon
	// must not be skipped: the property is part of the dictionary.
	line := `{"type": "property", "id": "P131", "labels": {"en": {"language": "en", "value": "located in"}}, "claims": {}}`

	stats, _ := runOne(t, line)

	if stats.Properties != 1 {
		t.Errorf("properties = %d, want 1", stats.Properties)
	}
	if stats.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", stats.Skipped)
	}
}

func TestHasTypeValue(t *testing.T) {
	cases := map[string]bool{
		`{"type":"property","id":"P1"}`:    true,
		`{"type": "property", "id": "P1"}`: true,
		`{"type"  :  "property"}`:          true,
		`{"id":"P1","type":"property"}`:    true,
		`{"type":"item","id":"Q1"}`:        false,
		`{"type": "item", "id": "Q1"}`:     false,
		`{"label":{"type":"property"}}`:    true,
		`{"type":"propertyx"}`:             false,
		`{"type":123}`:                     false,
		"{\"type\":\"property":             false,
	}

	for line, want := range cases {
		if got := hasTypeValue([]byte(line), propertyType); got != want {
			t.Errorf("hasTypeValue(%q) = %v, want %v", line, got, want)
		}
	}
}
