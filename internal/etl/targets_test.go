package etl

import (
	"testing"
)

func TestResolveTargetsKeepsASingleTargetAsItWas(t *testing.T) {
	// A single target keeps the file the user named: the command has
	// always behaved that way, and a run that is not over several
	// targets should not change.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs: []string{"Q90"},
		OutItems:   "paris-wikidata-subset.ndjson",
		OutProps:   "wikidata-properties.ndjson",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("there are %d targets, want 1", len(targets))
	}
	if targets[0].QID != "Q90" {
		t.Errorf("the target is %q, want Q90", targets[0].QID)
	}
	if targets[0].ItemsPath != "paris-wikidata-subset.ndjson" {
		t.Errorf("the file is %q, want the one the user named", targets[0].ItemsPath)
	}
}

func TestResolveTargetsDerivesTheFileOfEveryTarget(t *testing.T) {
	// The user names one file and two targets. The file of the second
	// target is derived from the first one, with the name of the target
	// in it.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs:   []string{"Q90", "Q148"},
		TargetLabels: []string{"paris", "shanghai"},
		OutItems:     "paris-wikidata-subset.ndjson",
		OutProps:     "wikidata-properties.ndjson",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("there are %d targets, want 2", len(targets))
	}

	if targets[0].ItemsPath != "paris-wikidata-subset.ndjson" {
		t.Errorf("the file of the first target is %q", targets[0].ItemsPath)
	}
	if targets[1].ItemsPath != "shanghai-wikidata-subset.ndjson" {
		t.Errorf("the file of the second target is %q, want shanghai-wikidata-subset.ndjson",
			targets[1].ItemsPath)
	}
}

func TestResolveTargetsGivesEveryTargetItsOwnFile(t *testing.T) {
	// The files of two targets must not be the same: the second one
	// would overwrite the first.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs: []string{"Q90", "Q148"},
		OutItems:   "items.ndjson",
		OutProps:   "props.ndjson",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if targets[0].ItemsPath == targets[1].ItemsPath {
		t.Errorf("the two targets share the file %q", targets[0].ItemsPath)
	}
}

func TestResolveTargetsUsesTheFilesTheUserNamed(t *testing.T) {
	// The user may name the file of every target.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs:        []string{"Q90", "Q148"},
		OutItems:          "ignored.ndjson",
		OutItemsPerTarget: []string{"first.ndjson", "second.ndjson"},
		OutProps:          "props.ndjson",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if targets[0].ItemsPath != "first.ndjson" {
		t.Errorf("the file of the first target is %q", targets[0].ItemsPath)
	}
	if targets[1].ItemsPath != "second.ndjson" {
		t.Errorf("the file of the second target is %q", targets[1].ItemsPath)
	}
}

func TestResolveTargetsDropsTheDuplicateTargets(t *testing.T) {
	// The same target twice would write the same file twice, and the
	// second run would overwrite the first.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs: []string{"Q90", " Q90 ", "Q148"},
		OutItems:   "items.ndjson",
		OutProps:   "props.ndjson",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("there are %d targets, want 2", len(targets))
	}
	if targets[0].QID != "Q90" || targets[1].QID != "Q148" {
		t.Errorf("the targets are %q and %q", targets[0].QID, targets[1].QID)
	}
}

func TestResolveTargetsRefusesMoreNamesThanTargets(t *testing.T) {
	// A name without a target is a mistake: it is reported rather
	// than ignored.
	_, err := ResolveTargets(CommandOptions{
		TargetQIDs:   []string{"Q90"},
		TargetLabels: []string{"paris", "shanghai"},
		OutItems:     "items.ndjson",
		OutProps:     "props.ndjson",
	})
	if err == nil {
		t.Fatal("the command accepted more names than targets")
	}
}

func TestResolveTargetsRefusesNoTargetAtAll(t *testing.T) {
	_, err := ResolveTargets(CommandOptions{
		TargetQIDs: []string{"  "},
		OutItems:   "items.ndjson",
		OutProps:   "props.ndjson",
	})
	if err == nil {
		t.Fatal("the command accepted a run without a target")
	}
}

func TestDeriveItemsPathKeepsTheExtension(t *testing.T) {
	cases := map[string]string{
		"paris-wikidata-subset.ndjson": "shanghai-wikidata-subset.ndjson",
		"items.ndjson":                 "shanghai-items.ndjson",
		"items":                        "shanghai-items",
	}

	for base, want := range cases {
		got := DeriveItemsPath(base, TargetSpec{QID: "Q148", Label: "shanghai"})
		if got != want {
			t.Errorf("DeriveItemsPath(%q) = %q, want %q", base, got, want)
		}
	}
}

func TestDeriveItemsPathUsesTheIdentifierWhenThereIsNoName(t *testing.T) {
	// A target without a name is named after its identifier.
	got := DeriveItemsPath("paris-wikidata-subset.ndjson", TargetSpec{QID: "Q148"})
	if got != "q148-wikidata-subset.ndjson" {
		t.Errorf("the file is %q, want q148-wikidata-subset.ndjson", got)
	}
}

func TestSlugifyMakesAFileNameSafe(t *testing.T) {
	cases := map[string]string{
		"shanghai":      "shanghai",
		"New York":      "new-york",
		"  paris  ":     "paris",
		"Q90":           "q90",
		"a/b\\c":        "a-b-c",
		"--weird--":     "weird",
		"":              "",
		"hong kong":     "hong-kong",
		"são paulo":     "s-o-paulo",
		"new_york city": "new_york-city",
	}

	for value, want := range cases {
		if got := slugify(value); got != want {
			t.Errorf("slugify(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestTheOptionsSayWhatTheRunWrites(t *testing.T) {
	// The items only and the properties only are what the flags say.
	both := CommandOptions{}
	if !both.WritesItems() || !both.WritesProperties() {
		t.Error("a plain run should write both")
	}

	items := CommandOptions{ItemsOnly: true}
	if !items.WritesItems() {
		t.Error("the items only run should write the items")
	}
	if items.WritesProperties() {
		t.Error("the items only run should not write the dictionary")
	}

	properties := CommandOptions{PropertiesOnly: true}
	if properties.WritesItems() {
		t.Error("the properties only run should not write the items")
	}
	if !properties.WritesProperties() {
		t.Error("the properties only run should write the dictionary")
	}
}

func TestResolveTargetsGivesNoFileToAPropertiesOnlyRun(t *testing.T) {
	// A run that writes the property dictionary only has no file of
	// items: there is nothing to write them to.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs:     []string{"Q90"},
		OutItems:       "items.ndjson",
		OutProps:       "props.ndjson",
		PropertiesOnly: true,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("there are %d targets, want 1", len(targets))
	}
	if targets[0].ItemsPath != "" {
		t.Errorf("the target has the file %q, want none", targets[0].ItemsPath)
	}
}

func TestResolveTargetsGivesAFileToAnItemsOnlyRun(t *testing.T) {
	// The items of an items only run are still written.
	targets, err := ResolveTargets(CommandOptions{
		TargetQIDs: []string{"Q90"},
		OutItems:   "items.ndjson",
		OutProps:   "props.ndjson",
		ItemsOnly:  true,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if targets[0].ItemsPath != "items.ndjson" {
		t.Errorf("the file of the target is %q, want items.ndjson", targets[0].ItemsPath)
	}
}
