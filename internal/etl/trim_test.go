package etl

import (
	"encoding/json"
	"testing"
)

func TestTrimEntityKeepsTheLanguagesItIsAskedFor(t *testing.T) {
	raw := &rawEntity{
		ID:   "Q90",
		Type: TypeItem,
		Labels: map[string]Term{
			"en": {Language: "en", Value: "Paris"},
			"fr": {Language: "fr", Value: "Paris"},
			"de": {Language: "de", Value: "Paris"},
			"es": {Language: "es", Value: "París"},
		},
		Descriptions: map[string]Term{
			"en": {Language: "en", Value: "The capital of France"},
			"fr": {Language: "fr", Value: "La capitale de la France"},
		},
		Aliases: map[string][]Term{
			"en": {{Language: "en", Value: "City of Light"}},
			"de": {{Language: "de", Value: "Lichtstadt"}},
		},
		Claims: map[string]json.RawMessage{},
	}

	trimmed := TrimEntity(raw, DefaultItemTrim())

	if len(trimmed.Labels) != 2 {
		t.Errorf("labels = %d, want 2", len(trimmed.Labels))
	}
	if _, ok := trimmed.Labels["en"]; !ok {
		t.Error("the English label was dropped")
	}
	if _, ok := trimmed.Labels["fr"]; !ok {
		t.Error("the French label was dropped")
	}
	if _, ok := trimmed.Labels["de"]; ok {
		t.Error("the German label was kept")
	}

	if len(trimmed.Descriptions) != 1 {
		t.Errorf("descriptions = %d, want 1", len(trimmed.Descriptions))
	}
	if _, ok := trimmed.Descriptions["fr"]; ok {
		t.Error("the French description was kept")
	}

	if len(trimmed.Aliases) != 1 {
		t.Errorf("aliases = %d, want 1", len(trimmed.Aliases))
	}
	if _, ok := trimmed.Aliases["de"]; ok {
		t.Error("the German alias was kept")
	}
}

func TestTrimEntityKeepsTheClaimsWhole(t *testing.T) {
	raw := &rawEntity{
		ID:   "Q90",
		Type: TypeItem,
		Claims: map[string]json.RawMessage{
			"P131": json.RawMessage(`[{"mainsnak":{"snaktype":"value"},"qualifiers":{"P580":[{}]}}]`),
		},
	}

	trimmed := TrimEntity(raw, DefaultItemTrim())

	claim, ok := trimmed.Claims["P131"]
	if !ok {
		t.Fatal("the claim was dropped")
	}
	if string(claim) != string(raw.Claims["P131"]) {
		t.Errorf("the claim was changed:\n got: %s\nwant: %s", claim, raw.Claims["P131"])
	}
}

func TestTrimEntityOmitsTheEmptyAliases(t *testing.T) {
	raw := &rawEntity{
		ID:      "P131",
		Type:    TypeProperty,
		Aliases: map[string][]Term{"de": {{Language: "de", Value: "x"}}},
		Claims:  map[string]json.RawMessage{},
	}

	trimmed := TrimEntity(raw, DefaultPropertyTrim())
	if trimmed.Aliases != nil {
		t.Errorf("aliases = %v, want none", trimmed.Aliases)
	}

	// The field is absent from the JSON, rather than null.
	encoded, err := json.Marshal(trimmed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["aliases"]; ok {
		t.Error("the aliases field is in the JSON, it should be absent")
	}
}

func TestTrimPropertyKeepsTheEnglishOnly(t *testing.T) {
	raw := &rawEntity{
		ID:   "P131",
		Type: TypeProperty,
		Labels: map[string]Term{
			"en": {Language: "en", Value: "located in the administrative territorial entity"},
			"fr": {Language: "fr", Value: "situé dans l'entité territoriale administrative"},
		},
		Descriptions: map[string]Term{
			"en": {Language: "en", Value: "A more specific place"},
		},
		Claims: map[string]json.RawMessage{},
	}

	trimmed := TrimEntity(raw, DefaultPropertyTrim())

	if len(trimmed.Labels) != 1 {
		t.Errorf("labels = %d, want 1", len(trimmed.Labels))
	}
	if len(trimmed.Descriptions) != 1 {
		t.Errorf("descriptions = %d, want 1", len(trimmed.Descriptions))
	}
}
