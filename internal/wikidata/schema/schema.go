// Package schema holds the shape of the trimmed Wikidata entities.
//
// It is the single source of truth for the records that travel between
// the two halves of the pipeline:
//
//	the etl worker  →  the NDJSON files  →  the PocketBase importer
//
// The worker writes the records, and the importer reads them back. If
// the two sides declared their own structures, they would drift, and a
// record the worker wrote would be read as an empty object.
//
// Nothing here talks to the database or to a file. The package is the
// vocabulary the two sides share.
package schema

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
// The dump writes the language and the text in the same object, and
// the panel shows the language of a fallback label, so both are kept.
type Term struct {
	// Language is the language code, such as "en" or "fr".
	Language string `json:"language"`
	// Value is the text.
	Value string `json:"value"`
}

// Entity is the trimmed form of a dump entity.
//
// The fields are the ones the application needs and nothing else. The
// claims are kept whole, qualifiers included, because a claim that
// carries a qualifier says something the plain value does not.
//
// The same structure is written to the NDJSON files and read back by
// the importer, so it carries the JSON tags of the file format.
type Entity struct {
	// ID is the identifier of the entity, such as "Q90" or "P131".
	ID string `json:"id"`
	// Type is the kind of entity: "item" or "property".
	Type string `json:"type"`
	// Labels holds the labels of the entity, by language.
	Labels map[string]Term `json:"labels"`
	// Descriptions holds the descriptions of the entity, by language.
	Descriptions map[string]Term `json:"descriptions"`
	// Aliases holds the aliases of the entity, by language.
	//
	// The field is absent for a property that has none, and it is
	// omitted rather than written as null.
	Aliases map[string][]Term `json:"aliases,omitempty"`
	// Claims holds the statements of the entity, by property.
	//
	// The statements are kept as they were written, so that the
	// application reads the same shape the dump has.
	Claims map[string]json.RawMessage `json:"claims"`
}
