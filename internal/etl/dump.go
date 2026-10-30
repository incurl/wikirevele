package etl

import (
	"encoding/json"

	"github.com/incurl/wikirevele/internal/wikidata/schema"
)

// rawEntity is the part of a dump line the pipeline reads.
//
// The dump holds a great deal more than this: sitelinks, references,
// every language variant, and a long tail of metadata. None of it is
// declared here, so that the decoder skips it instead of allocating
// for it.
//
// The claims are kept as raw JSON. Only the statements of the
// properties on the allowlist are decoded, and the ones that are kept
// are written back out unchanged.
type rawEntity struct {
	// ID is the identifier of the entity.
	ID string `json:"id"`
	// Type is the kind of entity.
	Type string `json:"type"`
	// Labels holds the labels, by language.
	Labels map[string]schema.Term `json:"labels"`
	// Descriptions holds the descriptions, by language.
	Descriptions map[string]schema.Term `json:"descriptions"`
	// Aliases holds the aliases, by language.
	Aliases map[string][]schema.Term `json:"aliases"`
	// Claims holds the statements, by property.
	Claims map[string]json.RawMessage `json:"claims"`
}

// rawStatement is the part of a statement the filter reads.
//
// A statement of the dump wraps its payload in a "mainsnak" object.
// The qualifiers, the references and the rank are not declared: they
// are not used to filter, and the ones that are kept are written back
// out unchanged.
type rawStatement struct {
	// MainSnak is the payload of the statement.
	MainSnak *rawSnak `json:"mainsnak"`
}

// rawSnak is the payload of a statement.
type rawSnak struct {
	// SnakeType is the kind of payload: "value", "novalue" or
	// "somevalue".
	SnakeType string `json:"snaktype"`
	// Property is the property the statement is about.
	Property string `json:"property"`
	// Datatype is the datatype of the property.
	Datatype string `json:"datatype"`
	// DataValue is the value of the statement.
	DataValue *rawDataValue `json:"datavalue"`
}

// rawDataValue is the value of a statement.
type rawDataValue struct {
	// Value holds the value itself. Its shape depends on the type.
	Value json.RawMessage `json:"value"`
	// Type names the kind of value.
	Type string `json:"type"`
}

// rawEntityValue is a value that points at another entity.
//
// It is the shape of a "wikibase-entityid" value.
type rawEntityValue struct {
	// EntityType is the kind of the entity: "item" or "property".
	EntityType string `json:"entity-type"`
	// ID is the identifier of the entity, such as "Q90".
	ID string `json:"id"`
}

// The snake types and the datatypes the filter cares about.
const (
	// SnakeTypeValue is the snake type of a statement that carries a
	// value.
	//
	// The other two, "novalue" and "somevalue", say that a link exists
	// or does not exist, not which entity it points at, so there is
	// nothing to match.
	SnakeTypeValue = "value"

	// DatatypeItem is the datatype of a property whose value is another
	// entity.
	DatatypeItem = "wikibase-item"

	// DataValueTypeEntityID is the type of a value that points at
	// another entity.
	DataValueTypeEntityID = "wikibase-entityid"
)
