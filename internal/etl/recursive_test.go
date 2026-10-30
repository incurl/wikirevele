package etl

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// p131Statement builds the JSON of a P131 statement that points at parent.
func p131Statement(t *testing.T, parent string) json.RawMessage {
	t.Helper()

	statement := map[string]any{
		"mainsnak": map[string]any{
			"snaktype": "value",
			"property": PropertyLocatedIn,
			"datatype": DatatypeItem,
			"datavalue": map[string]any{
				"value": map[string]any{
					"entity-type": "item",
					"id":          parent,
				},
				"type": DataValueTypeEntityID,
			},
		},
	}

	raw, err := json.Marshal([]any{statement})
	if err != nil {
		t.Fatalf("marshal the statement: %v", err)
	}

	return raw
}

// itemWith builds the JSON of an item that is located in parent.
func itemWith(t *testing.T, id, parent string) string {
	t.Helper()

	item := map[string]any{
		"type":   TypeItem,
		"id":     id,
		"labels": map[string]any{"en": map[string]any{"language": "en", "value": id}},
	}
	if parent != "" {
		item["claims"] = map[string]any{PropertyLocatedIn: p131Statement(t, parent)}
	}

	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal the item: %v", err)
	}

	return string(raw)
}

func TestTheRecursiveModeKeepsTheWholeChain(t *testing.T) {
	// Paris holds Lyon, and Lyon holds a district. The district does
	// not point at Paris: it points at Lyon, and it is still inside
	// Paris.
	dump := strings.Join([]string{
		itemWith(t, "Q90", ""),
		itemWith(t, "Q75", "Q90"),
		itemWith(t, "Q456", "Q75"),
	}, "\n")

	items, props := filepath.Join(t.TempDir(), "items.ndjson"), filepath.Join(t.TempDir(), "props.ndjson")

	ids := runRecursive(t, dump, "Q90", items, props)

	want := []string{"Q90", "Q75", "Q456"}
	assertSameIDs(t, ids, want)
}

func TestTheRecursiveModeDropsWhatIsOutsideTheTarget(t *testing.T) {
	// Berlin is in Germany, not in Paris.
	dump := strings.Join([]string{
		itemWith(t, "Q90", ""),
		itemWith(t, "Q75", "Q90"),
		itemWith(t, "Q999", "Q64"),
	}, "\n")

	dir := t.TempDir()
	items, props := filepath.Join(dir, "items.ndjson"), filepath.Join(dir, "props.ndjson")

	ids := runRecursive(t, dump, "Q90", items, props)

	assertSameIDs(t, ids, []string{"Q90", "Q75"})
}

func TestTheRecursiveModeDoesNotLoopOnACycle(t *testing.T) {
	// Two entities that point at each other would make the walk loop
	// forever if it did not remember where it has been.
	dump := strings.Join([]string{
		itemWith(t, "Q90", ""),
		itemWith(t, "Q1", "Q2"),
		itemWith(t, "Q2", "Q1"),
	}, "\n")

	dir := t.TempDir()
	items, props := filepath.Join(dir, "items.ndjson"), filepath.Join(dir, "props.ndjson")

	ids := runRecursive(t, dump, "Q90", items, props)

	assertSameIDs(t, ids, []string{"Q90"})
}

func TestThePlainModeKeepsOnlyWhatPointsAtTheTarget(t *testing.T) {
	// Without the flag, the chain is not walked: only the items that
	// point at the target are kept.
	dump := strings.Join([]string{
		itemWith(t, "Q90", ""),
		itemWith(t, "Q75", "Q90"),
		itemWith(t, "Q456", "Q75"),
	}, "\n")

	dir := t.TempDir()
	items, props := filepath.Join(dir, "items.ndjson"), filepath.Join(dir, "props.ndjson")

	ids := runPlain(t, dump, "Q90", items, props)

	assertSameIDs(t, ids, []string{"Q75"})
}

// runRecursive runs the pipeline in the recursive mode and returns the
// identifiers of the items that were written.
func runRecursive(t *testing.T, dump, target, itemsPath, propsPath string) []string {
	t.Helper()
	return runRecursivePipeline(t, dump, target, itemsPath, propsPath, true)
}

// runPlain runs the pipeline without the recursive mode.
func runPlain(t *testing.T, dump, target, itemsPath, propsPath string) []string {
	t.Helper()
	return runRecursivePipeline(t, dump, target, itemsPath, propsPath, false)
}

// runRecursivePipeline runs the pipeline and reads the items back.
func runRecursivePipeline(t *testing.T, dump, target, itemsPath, propsPath string, recursive bool) []string {
	t.Helper()

	items, err := os.Create(itemsPath)
	if err != nil {
		t.Fatalf("create the items: %v", err)
	}
	props, err := os.Create(propsPath)
	if err != nil {
		t.Fatalf("create the properties: %v", err)
	}

	pipeline := NewPipeline(Options{
		Items:      items,
		Properties: props,
		Target:     target,
		Recursive:  recursive,
	})

	if _, err := pipeline.Run(context.Background(), strings.NewReader(dump)); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := pipeline.Finish(itemsPath); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := pipeline.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	_ = items.Close()
	_ = props.Close()

	return readIDs(t, itemsPath)
}

// readIDs reads the identifiers of the items of a file.
func readIDs(t *testing.T, path string) []string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	ids := []string{}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var entity Entity
		if err := json.Unmarshal(line, &entity); err != nil {
			t.Fatalf("the line is not an entity: %v", err)
		}
		ids = append(ids, entity.ID)
	}

	return ids
}

// assertSameIDs checks the identifiers, in any order.
func assertSameIDs(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("the items are %v, want %v", got, want)
	}

	seen := make(map[string]struct{}, len(got))
	for _, id := range got {
		seen[id] = struct{}{}
	}
	for _, id := range want {
		if _, ok := seen[id]; !ok {
			t.Fatalf("the items are %v, want %v", got, want)
		}
	}
}
