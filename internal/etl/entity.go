// Package etl is the extraction and transformation layer of the
// Wikidata pipeline.
//
// The package reads the JSON Lines of a Wikidata dump, keeps the
// entities that are linked to Paris, trims them down to the fields the
// application needs, and writes them as NDJSON.
//
// The dump is enormous: the full one is more than a hundred gigabytes
// compressed. The package is therefore built around a single rule:
// exactly one entity is held in memory at a time. The scanner reads a
// line, the line is filtered and trimmed, the result is written, and
// the line is dropped. Nothing accumulates.
//
// The trimming happens here and nowhere else. The application that
// reads the output never sees a sitelink, a reference, or a language
// variant that was not asked for.
package etl

import (
	"github.com/incurl/wikirevele/internal/wikidata/schema"
)

// Term is a label, a description or an alias, in one language.
//
// It is the type of the shared schema: the worker writes it and the
// importer reads it back.
type Term = schema.Term

// Entity is the trimmed form of a dump entity.
//
// It is the type of the shared schema, so that the record the worker
// writes and the record the importer reads are the same one.
type Entity = schema.Entity

// The kinds of entity the dump holds.
//
// They are the kinds of the shared schema, for the same reason.
const (
	// TypeItem is a Q-entity, such as Q90.
	TypeItem = schema.TypeItem
	// TypeProperty is a P-entity, such as P131.
	TypeProperty = schema.TypeProperty
)
