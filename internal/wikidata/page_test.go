package wikidata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLines writes the given NDJSON lines to a file and returns its
// path.
func writeLines(t *testing.T, lines []string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "dataset.ndjson")

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write the dataset: %v", err)
	}

	return path
}

// importLines imports the given NDJSON lines, as the importer does.
func importLines(t *testing.T, lines ...string) *Service {
	t.Helper()

	service := newTestService(t)

	path := writeLines(t, lines)
	if err := service.ImportFile(KindItems, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	// The import runs in the background, so the test waits for it.
	waitForDone(t, service)

	return service
}

func TestThePageFiltersOnAClaim(t *testing.T) {
	// Two items located in Paris, one in Lyon.
	service := importLines(t,
		`{"id":"Q1","type":"item","labels":{"en":{"language":"en","value":"One"}},"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131","datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},"type":"wikibase-entityid"}}}]}}`,
		`{"id":"Q2","type":"item","labels":{"en":{"language":"en","value":"Two"}},"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131","datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q90"},"type":"wikibase-entityid"}}}]}}`,
		`{"id":"Q3","type":"item","labels":{"en":{"language":"en","value":"Three"}},"claims":{"P131":[{"mainsnak":{"snaktype":"value","property":"P131","datatype":"wikibase-item","datavalue":{"value":{"entity-type":"item","id":"Q456"},"type":"wikibase-entityid"}}}]}}`,
	)

	records, total, err := NewReader(service.App()).ListItemsPage(PageQuery{
		Limit:   50,
		Filters: []ClaimFilter{{Property: "P131", Value: "Q90"}},
	})
	if err != nil {
		t.Fatalf("page: %v", err)
	}

	if total != 2 {
		t.Errorf("the total is %d, want 2", total)
	}
	if len(records) != 2 {
		t.Fatalf("the page holds %d items, want 2", len(records))
	}

	for _, item := range records {
		if item.QID == "Q3" {
			t.Errorf("the item of Lyon is in the page")
		}
	}
}

func TestThePagePaginatesAndCounts(t *testing.T) {
	service := importLines(t,
		`{"id":"Q1","type":"item","labels":{},"claims":{}}`,
		`{"id":"Q2","type":"item","labels":{},"claims":{}}`,
		`{"id":"Q3","type":"item","labels":{},"claims":{}}`,
	)
	reader := NewReader(service.App())

	first, total, err := reader.ListItemsPage(PageQuery{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if total != 3 {
		t.Errorf("the total is %d, want 3", total)
	}
	if len(first) != 2 {
		t.Fatalf("the first page holds %d items, want 2", len(first))
	}

	second, _, err := reader.ListItemsPage(PageQuery{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("the second page holds %d items, want 1", len(second))
	}
	if second[0].QID != "Q3" {
		t.Errorf("the second page starts with %q, want Q3", second[0].QID)
	}
}

func TestThePageSortsByIdentifier(t *testing.T) {
	service := importLines(t,
		`{"id":"Q2","type":"item","labels":{},"claims":{}}`,
		`{"id":"Q1","type":"item","labels":{},"claims":{}}`,
	)

	records, _, err := NewReader(service.App()).ListItemsPage(PageQuery{
		Limit:          10,
		SortDescending: true,
	})
	if err != nil {
		t.Fatalf("page: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("the page holds %d items, want 2", len(records))
	}
	if records[0].QID != "Q2" {
		t.Errorf("the first item is %q, want Q2 in a descending order", records[0].QID)
	}
}

func TestTheFilterRefusesAPropertyThatIsNotAName(t *testing.T) {
	// The property of a filter is put into the expression of the
	// filter as it is. A property that is not a name of a property is
	// not put into it at all.
	service := importLines(t,
		`{"id":"Q1","type":"item","labels":{},"claims":{}}`,
	)

	reader := NewReader(service.App())

	cases := []string{
		`P131" || qid != "ZZZ`,
		`P131 or qid`,
		`P131 || qid`,
		`P131/sitelinks`,
		`P131 & qid`,
		`P131 = 1`,
		`P131.qid`,
	}

	for _, property := range cases {
		filter := reader.filterOf([]ClaimFilter{{Property: property, Value: "Q90"}})
		if filter != "" {
			t.Errorf("the property %q made the filter %q, want it refused", property, filter)
		}
	}
}

func TestTheFilterAcceptsAPropertyThatIsAName(t *testing.T) {
	service := importLines(t,
		`{"id":"Q1","type":"item","labels":{},"claims":{}}`,
	)

	reader := NewReader(service.App())

	filter := reader.filterOf([]ClaimFilter{{Property: "P131", Value: "Q90"}})
	if filter != `claims.P131 ~ "Q90"` {
		t.Errorf("the filter is %q, want it to name the property", filter)
	}
}

func TestTheFilterEscapesTheValue(t *testing.T) {
	service := importLines(t,
		`{"id":"Q1","type":"item","labels":{},"claims":{}}`,
	)

	reader := NewReader(service.App())

	filter := reader.filterOf([]ClaimFilter{{Property: "P131", Value: `Q90" || qid != "ZZZ`}})
	if strings.Contains(filter, `qid != "ZZZ ~ `) {
		t.Errorf("the filter is %q, want the value escaped", filter)
	}
	if !strings.Contains(filter, `\"`) {
		t.Errorf("the filter is %q, want the quotes of the value escaped", filter)
	}
}
