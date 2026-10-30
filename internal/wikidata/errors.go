package wikidata

import "errors"

// ErrNoIdentifier is returned for a record without an identifier.
//
// The identifier is the key the records are upserted by, so a record
// without one cannot be stored: it would collide with every other
// record without one.
var ErrNoIdentifier = errors.New("wikidata: the record has no identifier")

// ErrUnknownKind is returned for a record that is neither an item nor
// a property.
//
// The datasets hold two kinds of entity. A record of another kind is
// a file that is not one of them, and it is skipped.
var ErrUnknownKind = errors.New("wikidata: the record is neither an item nor a property")
