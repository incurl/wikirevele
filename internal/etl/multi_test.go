package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runTargets runs the pipeline over the given lines, with one target per
// entry of the map.
//
// It returns the identifiers that were written for every target, and
// the property dictionary.
func runTargets(t *testing.T, recursive bool, lines []string, targets map[string]string) (map[string][]string, string, Stats) {
	t.Helper()

	var properties bytes.Buffer
	var buffers map[string]*bytes.Buffer

	targets2 := make([]*Target, 0, len(targets))
	buffers = make(map[string]*bytes.Buffer, len(targets))

	for qid, label := range targets {
		buffer := &bytes.Buffer{}
		buffers[qid] = buffer
		targets2 = append(targets2, NewTarget(qid, label, "", NewEntityWriter(buffer), recursive))
	}

	pipeline := NewPipeline(Options{
		Targets:         targets2,
		Properties:      &properties,
		WriteProperties: true,
		Recursive:       recursive,
	})

	stats, err := pipeline.Run(context.Background(), strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	written := make(map[string][]string, len(buffers))
	for qid, buffer := range buffers {
		written[qid] = idsOf(t, buffer.String())
	}

	return written, properties.String(), stats
}

// idsOf reads the identifiers of an NDJSON text.
func idsOf(t *testing.T, text string) []string {
	t.Helper()

	ids := []string{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entity Entity
		if err := json.Unmarshal([]byte(line), &entity); err != nil {
			t.Fatalf("the line is not an entity: %v", err)
		}
		ids = append(ids, entity.ID)
	}

	return ids
}

// hasID tells whether a list holds an identifier.
func hasID(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// TestTheRunWritesEachItemToItsOwnTarget verifies that an item is
// written to the file of every target it is linked to.
//
// This is the point of a run over several targets: the dump is read
// once, and each item lands in the files of the targets that want it.
func TestTheRunWritesEachItemToItsOwnTarget(t *testing.T) {
	written, _, stats := runTargets(t, false, []string{
		// An item of Paris.
		itemLine("Q75", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
		// An item of Shanghai.
		itemLine("Q8686", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q148"),
		}),
		// An item of New York.
		itemLine("Q60", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q60"),
		}),
	}, map[string]string{
		"Q90":  "paris",
		"Q148": "shanghai",
		"Q60":  "newyork",
	})

	if !hasID(written["Q90"], "Q75") {
		t.Errorf("the item of Paris is missing: %v", written["Q90"])
	}
	if hasID(written["Q90"], "Q8686") {
		t.Errorf("the item of Shanghai is in the file of Paris: %v", written["Q90"])
	}

	if !hasID(written["Q148"], "Q8686") {
		t.Errorf("the item of Shanghai is missing: %v", written["Q148"])
	}
	if hasID(written["Q148"], "Q75") {
		t.Errorf("the item of Paris is in the file of Shanghai: %v", written["Q148"])
	}

	if !hasID(written["Q60"], "Q60") {
		t.Errorf("the item of New York is missing: %v", written["Q60"])
	}

	// Every item is counted once, even when it is written to several
	// files.
	if stats.Items != 3 {
		t.Errorf("items = %d, want 3", stats.Items)
	}
}

// TestAnItemCanBelongToSeveralTargets verifies that an item that
// points at two targets is written to both files.
//
// An entity that is located in two places is written to the file of
// each of them: the targets are independent, and the item belongs to
// both.
func TestAnItemCanBelongToSeveralTargets(t *testing.T) {
	written, _, _ := runTargets(t, false, []string{
		itemLine("Q1", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
			"P915": entityStatement("P915", "wikibase-item", "Q148"),
		}),
	}, map[string]string{
		"Q90":  "paris",
		"Q148": "shanghai",
	})

	if !hasID(written["Q90"], "Q1") {
		t.Errorf("the item is missing from the file of Paris: %v", written["Q90"])
	}
	if !hasID(written["Q148"], "Q1") {
		t.Errorf("the item is missing from the file of Shanghai: %v", written["Q148"])
	}
}

// TestTheRunCanWriteTheItemsOnly verifies that the property dictionary
// is not written when the run was asked for the items only.
func TestTheRunCanWriteTheItemsOnly(t *testing.T) {
	var items, properties bytes.Buffer

	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget(ParisQID, "", "", NewEntityWriter(&items), false),
		},
		Properties:      &properties,
		WriteProperties: false,
	})

	dump := strings.Join([]string{
		itemLine("Q75", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
		propertyLine("P131"),
	}, "\n")

	stats, err := pipeline.Run(context.Background(), strings.NewReader(dump))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if properties.Len() != 0 {
		t.Errorf("the dictionary was written: %q", properties.String())
	}
	if stats.Properties != 0 {
		t.Errorf("properties = %d, want 0", stats.Properties)
	}
	if stats.Items != 1 {
		t.Errorf("items = %d, want 1", stats.Items)
	}
}

// TestTheRunCanWriteThePropertiesOnly verifies that the items are not
// written when the run was asked for the dictionary only.
func TestTheRunCanWriteThePropertiesOnly(t *testing.T) {
	var items, properties bytes.Buffer

	pipeline := NewPipeline(Options{
		Targets:         nil,
		Properties:      &properties,
		WriteProperties: true,
	})

	dump := strings.Join([]string{
		itemLine("Q75", map[string]json.RawMessage{
			"P131": entityStatement("P131", "wikibase-item", "Q90"),
		}),
		propertyLine("P131"),
	}, "\n")

	stats, err := pipeline.Run(context.Background(), strings.NewReader(dump))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if items.Len() != 0 {
		t.Errorf("the items were written: %q", items.String())
	}
	if stats.Items != 0 {
		t.Errorf("items = %d, want 0", stats.Items)
	}
	if stats.Properties != 1 {
		t.Errorf("properties = %d, want 1", stats.Properties)
	}
}

// TestTheRecursiveModeKeepsEachTargetApart verifies that the
// hierarchies of two targets are walked separately.
//
// An item that is inside Paris is not inside Shanghai, and the
// recursive mode must not mix the two.
func TestTheRecursiveModeKeepsEachTargetApart(t *testing.T) {
	dir := t.TempDir()

	parisPath := filepath.Join(dir, "paris.ndjson")
	shanghaiPath := filepath.Join(dir, "shanghai.ndjson")

	parisFile, err := os.Create(parisPath)
	if err != nil {
		t.Fatalf("create the items of Paris: %v", err)
	}
	shanghaiFile, err := os.Create(shanghaiPath)
	if err != nil {
		t.Fatalf("create the items of Shanghai: %v", err)
	}

	pipeline := NewPipeline(Options{
		Targets: []*Target{
			NewTarget("Q90", "paris", parisPath, NewEntityWriter(parisFile), true),
			NewTarget("Q148", "shanghai", shanghaiPath, NewEntityWriter(shanghaiFile), true),
		},
		Properties:      io.Discard,
		WriteProperties: false,
		Recursive:       true,
	})

	// Paris holds Lyon, and Shanghai holds Pudong. Neither is inside
	// the other.
	dump := strings.Join([]string{
		itemWith(t, "Q90", ""),
		itemWith(t, "Q75", "Q90"),
		itemWith(t, "Q148", ""),
		itemWith(t, "Q8686", "Q148"),
	}, "\n")

	if _, err := pipeline.Run(context.Background(), strings.NewReader(dump)); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	_ = parisFile.Close()
	_ = shanghaiFile.Close()

	paris := idsOf(t, readFile(t, parisPath))
	shanghai := idsOf(t, readFile(t, shanghaiPath))

	if !hasID(paris, "Q75") {
		t.Errorf("the item of Paris is missing: %v", paris)
	}
	if hasID(paris, "Q8686") {
		t.Errorf("the item of Shanghai is in the file of Paris: %v", paris)
	}

	if !hasID(shanghai, "Q8686") {
		t.Errorf("the item of Shanghai is missing: %v", shanghai)
	}
	if hasID(shanghai, "Q75") {
		t.Errorf("the item of Paris is in the file of Shanghai: %v", shanghai)
	}
}

// readFile reads a whole file.
func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
