package wikidata

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/search"
)

// Reader reads the imported records back for the panel.
//
// The reader works on the records of the collections, so that the panel
// shows what was imported and nothing else. There is no file reading
// here: the file was read once, by the importer.
type Reader struct {
	// app is the PocketBase application the records are read from.
	app core.App
}

// NewReader creates a reader over the PocketBase application.
func NewReader(app core.App) *Reader {
	return &Reader{app: app}
}

// Item is one item of the dataset, as the panel reads it.
//
// The fields are the ones of the record in the collection, with the
// names of the Go structure, so that the frontend does not have to
// know the shape of the table.
type Item struct {
	// QID is the identifier of the item.
	QID string `json:"qid"`
	// Type is the kind of the entity.
	Type string `json:"type"`
	// Labels holds the labels of the item, by language.
	Labels map[string]Term `json:"labels"`
	// Descriptions holds the descriptions of the item, by language.
	Descriptions map[string]Term `json:"descriptions"`
	// Aliases holds the aliases of the item, by language.
	Aliases map[string][]Term `json:"aliases"`
	// Claims holds the statements of the item, by property.
	Claims map[string]json.RawMessage `json:"claims"`
}

// Property is one property of the dictionary, as the panel reads it.
type Property struct {
	// PID is the identifier of the property.
	PID string `json:"pid"`
	// Type is the kind of the entity.
	Type string `json:"type"`
	// Labels holds the labels of the property, by language.
	Labels map[string]Term `json:"labels"`
	// Descriptions holds the descriptions of the property, by language.
	Descriptions map[string]Term `json:"descriptions"`
	// Aliases holds the aliases of the property, by language.
	Aliases map[string][]Term `json:"aliases"`
	// Claims holds the statements of the property, by property.
	Claims map[string]json.RawMessage `json:"claims"`
}

// ListItems returns the items of the dataset.
//
// The records are read in the order of their identifier, so that two
// runs over the same data give the same list.
func (r *Reader) ListItems(limit, offset int) ([]Item, error) {
	records, err := r.app.FindRecordsByFilter(
		CollectionItems,
		"",     // no filter: every item
		"+qid", // ascending by the identifier
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("wikidata: list the items: %w", err)
	}

	items := make([]Item, 0, len(records))
	for _, record := range records {
		item, err := itemOf(record)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// ClaimFilter is a filter on the value of one property.
//
// The items of a dataset have no columns: their properties are claims,
// so a filter on a property is a filter on a claim. The value is the
// identifier the claim points at, or the text of the value.
type ClaimFilter struct {
	// Property is the property of the claim, such as "P131".
	Property string
	// Value is the value the claim must hold, such as "Q90".
	Value string
}

// PageQuery is a request for one page of items.
type PageQuery struct {
	// Limit is the number of items to return.
	Limit int
	// Offset is the number of items to skip.
	Offset int
	// SortField is the field to sort on: "qid" or "label".
	SortField string
	// SortDescending tells whether the order is reversed.
	SortDescending bool
	// Filters are the claims the items must hold.
	Filters []ClaimFilter
}

// ListItemsPage returns one page of the items, filtered, sorted and
// counted by the database.
//
// The filtering, the sorting and the counting happen in the query and
// not in the caller: the panel walks a dataset of a whole city, and
// reading all of it to keep the twentieth part would not scale.
//
// The filter on a claim is a filter on the nested key of the JSON
// column that holds the claims. It is a "contains" and not an equals:
// the value of a claim is an object, and the identifier is one of its
// fields.
func (r *Reader) ListItemsPage(query PageQuery) ([]Item, int64, error) {
	filter := r.filterOf(query.Filters)

	total, err := r.countFiltered(CollectionItems, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("wikidata: count the items: %w", err)
	}

	records, err := r.app.FindRecordsByFilter(
		CollectionItems,
		filter,
		sortClause(query),
		query.Limit,
		query.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("wikidata: list the items: %w", err)
	}

	items := make([]Item, 0, len(records))
	for _, record := range records {
		item, err := itemOf(record)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, int64(total), nil
}

// countFiltered counts the records of a collection that match a
// filter.
//
// The filter is the one PocketBase uses in its own queries, so the
// condition of the count is the condition of the page: a count that
// disagreed with the page would make the pagination of the panel
// wrong.
func (r *Reader) countFiltered(collectionName, filter string) (int64, error) {
	collection, err := r.app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return 0, err
	}

	if filter == "" {
		return r.count(collectionName)
	}

	resolver := core.NewRecordFieldResolver(r.app, collection, nil, true)
	expr, err := search.FilterData(filter).BuildExpr(resolver)
	if err != nil {
		return 0, fmt.Errorf("invalid filter expression: %w", err)
	}

	return r.app.CountRecords(collection, expr)
}

// filterOf builds the filter of a query.
//
// Every claim the query asks for has to be held, so the conditions are
// joined with "and". A value that holds a character the filter syntax
// reads is quoted, so that it cannot be read as part of the syntax.
func (r *Reader) filterOf(filters []ClaimFilter) string {
	conditions := make([]string, 0, len(filters))

	for _, filter := range filters {
		if filter.Property == "" || filter.Value == "" {
			continue
		}
		if !isPropertyName(filter.Property) {
			// A property that is not a name is refused here and not
			// in the parser: the parser would refuse it too, but
			// the error would reach the user as a failed page.
			continue
		}
		conditions = append(conditions, fmt.Sprintf(
			"claims.%s ~ %s", filter.Property, quoteValue(filter.Value)))
	}

	return strings.Join(conditions, " && ")
}

// isPropertyName tells whether a string can be a property of a claim.
//
// The property is put into the filter expression as it is, and the
// filter expression is read by the parser of PocketBase. A property is
// a name of a property of Wikidata, so it is letters and digits and
// the dash: "P31", "P131", "P366". Anything else is not a property,
// and it does not go into the expression.
//
// The value of a filter is quoted and escaped, so a value cannot break
// out of its string. The property is a name and not a string, and it
// cannot be quoted.
func isPropertyName(property string) bool {
	if property == "" {
		return false
	}

	for _, r := range property {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}

	return true
}

// sortClause builds the order of a query.
//
// The identifier is the order of the dataset, and it is the default.
func sortClause(query PageQuery) string {
	direction := "+"
	if query.SortDescending {
		direction = "-"
	}

	switch query.SortField {
	case "label":
		// The label is a JSON column of the languages, and the
		// database cannot order on it without an index. The
		// identifier is the stable order the panel shows.
		return direction + "qid"
	default:
		return direction + "qid"
	}
}

// quoteValue wraps a value for the filter syntax.
//
// The quotes are not optional: the parser reads a bare word as the name
// of a field, so an unquoted Q90 is looked up as a column and the
// filter is refused. The backslashes and the double quotes of the value
// are escaped, so that a value cannot break out of the string.
func quoteValue(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)

	return `"` + escaped + `"`
}

// CountItems returns the number of items in the dataset.
func (r *Reader) CountItems() (int64, error) {
	return r.count(CollectionItems)
}

// ListProperties returns the property dictionary.
func (r *Reader) ListProperties(limit, offset int) ([]Property, error) {
	records, err := r.app.FindRecordsByFilter(
		CollectionProperties,
		"",     // no filter: every property
		"+pid", // ascending by the identifier
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("wikidata: list the properties: %w", err)
	}

	properties := make([]Property, 0, len(records))
	for _, record := range records {
		property, err := propertyOf(record)
		if err != nil {
			return nil, err
		}
		properties = append(properties, property)
	}

	return properties, nil
}

// CountProperties returns the number of properties in the dictionary.
func (r *Reader) CountProperties() (int64, error) {
	return r.count(CollectionProperties)
}

// count returns the number of records in a collection.
//
// The count is asked of the database: loading every record to count
// them would be slow and would hold the whole dataset in memory.
func (r *Reader) count(collection string) (int64, error) {
	var count int64
	err := r.app.DB().
		NewQuery("SELECT COUNT(*) FROM " + collection).
		Row(&count)
	if err != nil {
		return 0, fmt.Errorf("wikidata: count the records of %s: %w", collection, err)
	}
	return count, nil
}

// itemOf turns a record into an item.
func itemOf(record *core.Record) (Item, error) {
	item := Item{
		QID:  record.GetString(FieldQID),
		Type: record.GetString(FieldType),
	}

	labels, err := decodeTerms(record, FieldLabels)
	if err != nil {
		return Item{}, err
	}
	item.Labels = labels

	descriptions, err := decodeTerms(record, FieldDescriptions)
	if err != nil {
		return Item{}, err
	}
	item.Descriptions = descriptions

	aliases, err := decodeAliases(record, FieldAliases)
	if err != nil {
		return Item{}, err
	}
	item.Aliases = aliases

	claims, err := decodeClaims(record, FieldClaims)
	if err != nil {
		return Item{}, err
	}
	item.Claims = claims

	return item, nil
}

// propertyOf turns a record into a property.
func propertyOf(record *core.Record) (Property, error) {
	property := Property{
		PID:  record.GetString(FieldPID),
		Type: record.GetString(FieldType),
	}

	labels, err := decodeTerms(record, FieldLabels)
	if err != nil {
		return Property{}, err
	}
	property.Labels = labels

	descriptions, err := decodeTerms(record, FieldDescriptions)
	if err != nil {
		return Property{}, err
	}
	property.Descriptions = descriptions

	aliases, err := decodeAliases(record, FieldAliases)
	if err != nil {
		return Property{}, err
	}
	property.Aliases = aliases

	claims, err := decodeClaims(record, FieldClaims)
	if err != nil {
		return Property{}, err
	}
	property.Claims = claims

	return property, nil
}

// decodeTerms reads the labels or the descriptions of a record.
//
// The values are stored as JSON, so they are decoded into the terms.
// A record without the field is empty, and a value that does not
// decode is a damaged record: it is reported rather than dropped.
func decodeTerms(record *core.Record, field string) (map[string]Term, error) {
	raw, err := jsonField(record, field)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]Term{}, nil
	}

	var terms map[string]Term
	if err := json.Unmarshal(raw, &terms); err != nil {
		return nil, fmt.Errorf("wikidata: decode the %s of %s: %w", field, record.GetString(FieldQID), err)
	}

	return terms, nil
}

// decodeAliases reads the aliases of a record.
func decodeAliases(record *core.Record, field string) (map[string][]Term, error) {
	raw, err := jsonField(record, field)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string][]Term{}, nil
	}

	var aliases map[string][]Term
	if err := json.Unmarshal(raw, &aliases); err != nil {
		return nil, fmt.Errorf("wikidata: decode the %s of %s: %w", field, record.GetString(FieldQID), err)
	}

	return aliases, nil
}

// decodeClaims reads the statements of a record.
func decodeClaims(record *core.Record, field string) (map[string]json.RawMessage, error) {
	raw, err := jsonField(record, field)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}

	var claims map[string]json.RawMessage
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf("wikidata: decode the %s of %s: %w", field, record.GetString(FieldQID), err)
	}

	return claims, nil
}

// jsonField reads a JSON field of a record.
//
// The value of a JSON field is a string that holds the JSON, so it is
// unwrapped before it is decoded.
func jsonField(record *core.Record, field string) ([]byte, error) {
	value := record.Get(field)
	if value == nil {
		return nil, nil
	}

	switch typed := value.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("wikidata: read the %s of %s: %w", field, record.GetString(FieldQID), err)
		}
		return raw, nil
	}
}
