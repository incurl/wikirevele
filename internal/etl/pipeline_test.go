package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A dump line for an item.
func itemLine(id string, claims map[string]json.RawMessage) string {
	entity := map[string]any{
		"type":         TypeItem,
		"id":           id,
		"labels":       map[string]any{"en": map[string]any{"language": "en", "value": id}},
		"descriptions": map[string]any{"en": map[string]any{"language": "en", "value": "an item"}},
		"claims":       claims,
		"sitelinks":    map[string]any{"enwiki": map[string]any{"site": "enwiki", "title": id}},
	}
	raw, _ := json.Marshal(entity)
	return string(raw)
}

// A dump line for a property.
func propertyLine(id string) string {
	entity := map[string]any{
		"type":   TypeProperty,
		"id":     id,
		"labels": map[string]any{"en": map[string]any{"language": "en", "value": id}},
		"claims": map[string]any{},
	}
	raw, _ := json.Marshal(entity)
	return string(raw)
}

// A dump line for an entity the pipeline skips.
func skippedLine(kind string) string {
	raw, _ := json.Marshal(map[string]any{"type": kind, "id": "L1", "claims": map[string]any{}})
	return string(raw)
}

// runPipeline runs the pipeline over the given lines.
//
// It is the single target case: one items file and one property file.
func runPipeline(t *testing.T, lines ...string) (string, string, Stats) {
	t.Helper()

	var items, properties bytes.Buffer

	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget(ParisQID, "", "", NewEntityWriter(&items), false),
		},
		Properties:      &properties,
		WriteProperties: true,
	})

	stats, err := pipeline.Run(context.Background(), strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	return items.String(), properties.String(), stats
}

func TestPipelineKeepsTheItemsThatPointAtParis(t *testing.T) {
	items, _, stats := runPipeline(t,
		itemLine("Q90", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
		itemLine("Q183", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q183"),
		}),
	)

	if !strings.Contains(items, `"Q90"`) {
		t.Errorf("the item that points at Paris is missing:\n%s", items)
	}
	if strings.Contains(items, `"Q183"`) {
		t.Errorf("the item that points at France was kept:\n%s", items)
	}
	if stats.Items != 1 {
		t.Errorf("items = %d, want 1", stats.Items)
	}
	if stats.Scanned != 2 {
		t.Errorf("scanned = %d, want 2", stats.Scanned)
	}
}

func TestPipelineKeepsEveryProperty(t *testing.T) {
	_, properties, stats := runPipeline(t,
		propertyLine("P131"),
		propertyLine("P17"),
	)

	if !strings.Contains(properties, `"P131"`) || !strings.Contains(properties, `"P17"`) {
		t.Errorf("the properties are missing:\n%s", properties)
	}
	if stats.Properties != 2 {
		t.Errorf("properties = %d, want 2", stats.Properties)
	}
}

func TestPipelineSkipsTheOtherEntityKinds(t *testing.T) {
	_, _, stats := runPipeline(t,
		skippedLine("lexeme"),
		skippedLine("mediainfo"),
		skippedLine("somethingelse"),
	)

	if stats.Skipped != 3 {
		t.Errorf("skipped = %d, want 3", stats.Skipped)
	}
	if stats.Items != 0 || stats.Properties != 0 {
		t.Errorf("items = %d, properties = %d, want 0 and 0", stats.Items, stats.Properties)
	}
}

func TestPipelineSkipsADamagedLine(t *testing.T) {
	items, _, stats := runPipeline(t,
		"{not json",
		itemLine("Q90", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
	)

	if stats.Malformed != 1 {
		t.Errorf("malformed = %d, want 1", stats.Malformed)
	}
	if stats.Items != 1 {
		t.Errorf("items = %d, want 1", stats.Items)
	}
	if !strings.Contains(items, `"Q90"`) {
		t.Errorf("the run stopped at the damaged line:\n%s", items)
	}
}

func TestPipelineHandlesTheEntitiesInOnePass(t *testing.T) {
	// The items and the properties are read in the same pass, so a
	// property that comes after an item is still written.
	items, properties, stats := runPipeline(t,
		itemLine("Q90", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
		propertyLine("P131"),
	)

	if stats.Items != 1 || stats.Properties != 1 {
		t.Errorf("items = %d, properties = %d, want 1 and 1", stats.Items, stats.Properties)
	}
	if !strings.Contains(items, `"Q90"`) {
		t.Errorf("the item is missing:\n%s", items)
	}
	if !strings.Contains(properties, `"P131"`) {
		t.Errorf("the property is missing:\n%s", properties)
	}
}

func TestPipelineWritesOneEntityPerLine(t *testing.T) {
	items, _, _ := runPipeline(t,
		itemLine("Q90", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
		itemLine("Q243", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
	)

	lines := strings.Split(strings.TrimSpace(items), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	for _, line := range lines {
		var entity Entity
		if err := json.Unmarshal([]byte(line), &entity); err != nil {
			t.Errorf("the line is not a JSON object: %v", err)
		}
		if entity.ID == "" {
			t.Error("the entity has no identifier")
		}
	}
}

func TestPipelineStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var items, properties bytes.Buffer
	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget(ParisQID, "", "", NewEntityWriter(&items), false),
		},
		Properties:      &properties,
		WriteProperties: true,
	})

	_, err := pipeline.Run(ctx, strings.NewReader(itemLine("Q90", map[string]json.RawMessage{
		"P131": entityStatement("P131", "wikibase-item", "Q90"),
	})))

	if err == nil {
		t.Error("the run should have been stopped")
	}
}
