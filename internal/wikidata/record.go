// Package wikidata imports the trimmed Wikidata datasets into
// PocketBase, and reads them back for the panel.
//
// The package is deliberately free of any dump processing: it does not
// decompress anything, it does not read a Wikidata dump, and it does
// not filter entities. That is the job of the standalone ETL command,
// which writes the two NDJSON files this package imports:
//
//	paris-wikidata-subset.ndjson
//	wikidata-properties.ndjson
//
// The rule of the repository is that the trimming happens once, in the
// ETL stage. The records this package reads are already trimmed, and
// they are stored as they are.
package wikidata

import "encoding/json"

// The kinds of entity the datasets hold.
const (
	// TypeItem is a Q-entity, such as Q90.
	TypeItem = "item"
	// TypeProperty is a P-entity, such as P131.
	TypeProperty = "property"
)

// Term is a label, a description or an alias, in one language.
//
// The panel shows the language of a fallback label, so the language is
// part of the record.
type Term struct {
	// Language is the language code, such as "en" or "fr".
	Language string `json:"language"`
	// Value is the text.
	Value string `json:"value"`
}

// ItemRecord is one item of the trimmed dataset, as it is stored.
//
// The shape is the shape of the NDJSON line: the identifier, the kind,
// the terms and the claims. The claims are kept as they were written,
// so that the panel reads the same structure the ETL command produced.
type ItemRecord struct {
	// ID is the identifier of the item, such as "Q90".
	ID string `json:"id"`
	// Type is the kind of entity: "item" or "property".
	Type string `json:"type"`
	// Labels holds the labels of the item, by language.
	Labels map[string]Term `json:"labels"`
	// Descriptions holds the descriptions of the item, by language.
	Descriptions map[string]Term `json:"descriptions"`
	// Aliases holds the aliases of the item, by language.
	Aliases map[string][]Term `json:"aliases,omitempty"`
	// Claims holds the statements of the item, by property.
	Claims map[string]json.RawMessage `json:"claims"`
}

// PropertyRecord is one property of the trimmed dataset.
//
// The shape is the shape of the item: a property is an entity with an
// identifier of its own.
type PropertyRecord struct {
	// ID is the identifier of the property, such as "P131".
	ID string `json:"id"`
	// Type is the kind of entity: "item" or "property".
	Type string `json:"type"`
	// Labels holds the labels of the property, by language.
	Labels map[string]Term `json:"labels"`
	// Descriptions holds the descriptions of the property, by language.
	Descriptions map[string]Term `json:"descriptions"`
	// Aliases holds the aliases of the property, by language.
	Aliases map[string][]Term `json:"aliases,omitempty"`
	// Claims holds the statements of the property, by property.
	Claims map[string]json.RawMessage `json:"claims"`
}

// Validate tells whether a record can be stored.
//
// A record without an identifier is not storable: the identifier is the
// key the records are upserted by, and a record without one would be a
// duplicate of every other record without one.
func (r *ItemRecord) Validate() error {
	if r.ID == "" {
		return ErrNoIdentifier
	}
	return nil
}

// Validate tells whether a record can be stored.
func (r *PropertyRecord) Validate() error {
	if r.ID == "" {
		return ErrNoIdentifier
	}
	return nil
}
