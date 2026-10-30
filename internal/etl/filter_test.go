package etl

import (
	"encoding/json"
	"testing"
)

// A statement that points at an entity.
func entityStatement(property, datatype, value string) json.RawMessage {
	statement := map[string]any{
		"mainsnak": map[string]any{
			"snaktype": "value",
			"property": property,
			"datatype": datatype,
			"datavalue": map[string]any{
				"value": map[string]any{
					"entity-type": "item",
					"id":          value,
				},
				"type": "wikibase-entityid",
			},
		},
		"type": "statement",
		"rank": "normal",
	}
	raw, _ := json.Marshal([]any{statement})
	return raw
}

// A statement with no value.
func emptyStatement(property, datatype string) json.RawMessage {
	statement := map[string]any{
		"mainsnak": map[string]any{
			"snaktype": "novalue",
			"property": property,
			"datatype": datatype,
		},
		"type": "statement",
		"rank": "normal",
	}
	raw, _ := json.Marshal([]any{statement})
	return raw
}

// An entity with the given claims.
func entityWith(claims map[string]json.RawMessage) *rawEntity {
	return &rawEntity{ID: "Q1", Type: TypeItem, Claims: claims}
}

func TestLinksToTargetMatchesTheTarget(t *testing.T) {
	entity := entityWith(map[string]json.RawMessage{
		"P131": entityStatement("P131", "wikibase-item", "Q90"),
	})

	if !LinksToTarget(entity, ParisQID) {
		t.Error("the item points at Paris, it should have matched")
	}
}

func TestLinksToTargetIgnoresAnotherTarget(t *testing.T) {
	entity := entityWith(map[string]json.RawMessage{
		"P131": entityStatement("P131", "wikibase-item", "Q183"),
	})

	if LinksToTarget(entity, ParisQID) {
		t.Error("the item points at France, it should not have matched")
	}
}

func TestLinksToTargetNeedsTheAllowlist(t *testing.T) {
	// P17 is the country, and it is not on the allowlist.
	entity := entityWith(map[string]json.RawMessage{
		"P17": entityStatement("P17", "wikibase-item", "Q90"),
	})

	if LinksToTarget(entity, ParisQID) {
		t.Error("P17 is not on the allowlist, it should not have matched")
	}
}

func TestLinksToTargetNeedsTheItemDatatype(t *testing.T) {
	// A string value that happens to mention Paris is not a link.
	entity := entityWith(map[string]json.RawMessage{
		"P131": entityStatement("P131", "string", "Q90"),
	})

	if LinksToTarget(entity, ParisQID) {
		t.Error("the datatype is not wikibase-item, it should not have matched")
	}
}

func TestLinksToTargetNeedsAValue(t *testing.T) {
	entity := entityWith(map[string]json.RawMessage{
		"P131": emptyStatement("P131", "wikibase-item"),
	})

	if LinksToTarget(entity, ParisQID) {
		t.Error("the statement carries no value, it should not have matched")
	}
}

func TestLinksToTargetAcceptsEveryPropertyOfTheAllowlist(t *testing.T) {
	for property := range LocationProperties {
		entity := entityWith(map[string]json.RawMessage{
			property: entityStatement(property, "wikibase-item", "Q90"),
		})

		if !LinksToTarget(entity, ParisQID) {
			t.Errorf("%s is on the allowlist, it should have matched", property)
		}
	}
}

func TestLinksToTargetIgnoresTheQualifiers(t *testing.T) {
	// The statement points at Paris and carries a qualifier that says
	// otherwise. The qualifiers are not read, so it still matches.
	statement := map[string]any{
		"mainsnak": map[string]any{
			"snaktype": "value",
			"property": "P131",
			"datatype": "wikibase-item",
			"datavalue": map[string]any{
				"value": map[string]any{
					"entity-type": "item",
					"id":          "Q90",
				},
				"type": "wikibase-entityid",
			},
		},
		"qualifiers": map[string]any{
			"P582": []any{map[string]any{
				"snaktype": "value",
				"property": "P582",
				"datatype": "time",
			}},
		},
		"type": "statement",
		"rank": "normal",
	}
	raw, _ := json.Marshal([]any{statement})

	entity := entityWith(map[string]json.RawMessage{"P131": raw})
	if !LinksToTarget(entity, ParisQID) {
		t.Error("the qualifiers are not read, the statement should have matched")
	}
}

func TestLinksToTargetSurvivesADamagedClaim(t *testing.T) {
	entity := entityWith(map[string]json.RawMessage{
		"P131": json.RawMessage(`{"not": "an array"}`),
	})

	// A damaged claim is not a match, and it is not a crash.
	if LinksToTarget(entity, ParisQID) {
		t.Error("the claim is damaged, it should not have matched")
	}
}
