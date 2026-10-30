package etl

import "encoding/json"

// ParisQID is the identifier of Paris.
//
// It is the default target of the pipeline: an item is kept when one
// of its statements points at it.
const ParisQID = "Q90"

// LocationProperties is the allowlist of the properties that can link
// an entity to Paris.
//
// A statement is only read when its property is on this list. The list
// is the union of the properties that say where something is, where it
// was made, where it comes from, and where it was described:
//
//	P131  located in the administrative territorial entity
//	P915  filming location
//	P1448 official name
//	P1071 location of creation
//	P1552 has quality
//	P291 place of publication
//	P159 headquarters location
//	P551 residence
//	P1037 director/manager of
//	P19  place of birth
//	P20  place of death
//
// A property that is not on the list is not read at all, which is what
// keeps the pass fast over a dump of a hundred gigabytes.
var LocationProperties = map[string]struct{}{
	"P131":  {},
	"P915":  {},
	"P1448": {},
	"P1071": {},
	"P1552": {},
	"P291":  {},
	"P159":  {},
	"P551":  {},
	"P1037": {},
	"P19":   {},
	"P20":   {},
}

// IsLocationProperty tells whether a property can link an entity to
// Paris.
func IsLocationProperty(property string) bool {
	_, ok := LocationProperties[property]
	return ok
}

// LinksToTarget tells whether an entity carries a statement that
// points at the target.
//
// The statement must satisfy every rule:
//
//   - its property is on the allowlist,
//   - it carries a value, and not a "novalue" or a "somevalue",
//   - its datatype is "wikibase-item",
//   - its value is an entity identifier equal to the target.
//
// The qualifiers of the statement are not read. A statement that
// points at the target with a qualifier that contradicts it is still a
// statement that points at the target.
func LinksToTarget(entity *rawEntity, target string) bool {
	for property, raw := range entity.Claims {
		if !IsLocationProperty(property) {
			continue
		}

		var statements []rawStatement
		if err := json.Unmarshal(raw, &statements); err != nil {
			// A claim that cannot be read is not a match. The
			// dump is large and a damaged claim is rare, but it
			// must not stop the pass.
			continue
		}

		for _, statement := range statements {
			if statementMatches(statement, target) {
				return true
			}
		}
	}

	return false
}

// statementMatches tells whether one statement points at the target.
func statementMatches(statement rawStatement, target string) bool {
	snak := statement.MainSnak
	if snak == nil {
		return false
	}
	if snak.SnakeType != SnakeTypeValue {
		return false
	}
	if snak.Datatype != DatatypeItem {
		return false
	}
	if snak.DataValue == nil {
		return false
	}
	if snak.DataValue.Type != DataValueTypeEntityID {
		return false
	}

	var value rawEntityValue
	if err := json.Unmarshal(snak.DataValue.Value, &value); err != nil {
		return false
	}

	return value.ID == target
}
