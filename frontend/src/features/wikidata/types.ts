/**
 * The shape of a Wikidata item, as it appears in a dump.
 *
 * These types mirror the JSON of the dump exactly. They are the raw
 * layer: nothing here is derived, and nothing here is meant to be
 * rendered directly. The presentation layer reads the view model of
 * `view.ts` instead, which is built from these types.
 *
 * The reference is the canonical Wikibase JSON documentation:
 * https://doc.wikimedia.org/Wikibase/master/php/docs_topics_json.html
 *
 * The values are modelled as closed unions, so that a malformed value
 * is a type error at the parsing boundary rather than a crash in a
 * component.
 */

/** The kind of entity an item is. */
export type EntityType = "item" | "property";

/** The identifier of an entity, such as "Q90" or "P131". */
export type EntityId = string;

/**
 * A term of an entity: a label, a description or an alias.
 *
 * The language is the key of the map the term is held in, and is
 * repeated here because the dump carries it.
 */
export interface Term {
  /** The language of the term, such as "en" or "fr". */
  language: string;
  /** The text of the term. */
  value: string;
}

/**
 * The kind of a snak.
 *
 * `value` is the only one that carries a value: `novalue` and
 * `somevalue` say that a link exists or does not exist, not which
 * entity it points at.
 */
export type SnakeType = "value" | "novalue" | "somevalue";

/**
 * The kind of a value, as the discriminator of `datavalue.type`.
 *
 * This is the *value* kind. The property datatype is a different
 * discriminator: `snak.datatype`.
 */
export type DataValueType =
  | "wikibase-entityid"
  | "string"
  | "monolingualtext"
  | "time"
  | "quantity"
  | "globecoordinate"
  | "external-id";

/**
 * The datatype of a property, as the discriminator of `snak.datatype`.
 *
 * This is the *property* datatype, and it is what says how the value
 * should be read: a `url` property holds a string, a `wikibase-item`
 * property holds an entity.
 */
export type PropertyDatatype =
  | "wikibase-item"
  | "wikibase-property"
  | "string"
  | "url"
  | "external-id"
  | "monolingualtext"
  | "time"
  | "quantity"
  | "globecoordinate"
  | "wikibase-form"
  | "wikibase-sense"
  | "tabular-data"
  | "math"
  | "musical-notation"
  | "geo-shape"
  | "wikibase-lexeme"
  | "wikibase-mediainfo"
  | "unknown-type";

/** The rank of a statement. */
export type StatementRank = "preferred" | "normal" | "deprecated";

/** A value that points at another entity. */
export interface EntityIdValue {
  /** The kind of entity: "item" or "property". */
  "entity-type": "item" | "property";
  /** The identifier of the entity, such as "Q90". */
  id: EntityId;
  /** The numeric form of the identifier, as the dump carries it. */
  "numeric-id"?: number;
}

/** A value that is text in one language. */
export interface MonolingualTextValue {
  /** The text. */
  text: string;
  /** The language of the text, such as "fr". */
  language: string;
}

/**
 * A point in time.
 *
 * The `time` field is a string with a leading `+` or `-` and a year
 * that may be padded to more than four digits. The precision says how
 * much of the string is meaningful.
 */
export interface TimeValue {
  /** The moment, such as "+00000002014-01-01T00:00:00Z". */
  time: string;
  /** The precision: 0 is a year, 11 is a second. */
  precision: number;
  /** The offset from UTC, in minutes. */
  timezone?: number;
  /** Non-zero when the date is before the earliest known date. */
  before?: number;
  /** Non-zero when the date is after the latest known date. */
  after?: number;
  /** The calendar model, as a URL. */
  calendarmodel?: string;
}

/** A quantity: an amount and the unit it is counted in. */
export interface QuantityValue {
  /** The amount, as an arbitrary-precision decimal string. */
  amount: string;
  /** The unit, as a URL. */
  unit: string;
}

/** A point on the globe. */
export interface GlobeCoordinateValue {
  /** The latitude, in degrees. */
  latitude: number;
  /** The longitude, in degrees. */
  longitude: number;
  /** The globe the point is on, as a URL. */
  globe?: string;
  /** The precision, in degrees. */
  precision?: number;
}

/**
 * The value of a snak, without its kind.
 *
 * The kind is the `type` of the container that holds the value, so the
 * value itself carries no discriminator.
 */
export type DataValue =
  | EntityIdValue
  | string
  | MonolingualTextValue
  | TimeValue
  | QuantityValue
  | GlobeCoordinateValue;

/**
 * The value of a snak, with its kind.
 *
 * The `type` field says which kind of value the `value` field holds. It
 * is a sibling of the value, not a field of it: the dump writes
 * `"datavalue": { "value": { ... }, "type": "wikibase-entityid" }`.
 *
 * The union is what makes the renderers exhaustive: a renderer that
 * forgets a case does not compile.
 */
export type DataValueContainer =
  | { type: "wikibase-entityid"; value: EntityIdValue }
  | { type: "string"; value: string }
  | { type: "monolingualtext"; value: MonolingualTextValue }
  | { type: "time"; value: TimeValue }
  | { type: "quantity"; value: QuantityValue }
  | { type: "globecoordinate"; value: GlobeCoordinateValue }
  | { type: "external-id"; value: string };

/**
 * A snak: how to read a value, and the value itself.
 *
 * A snak is what a statement holds. The qualifiers and the references
 * hold snaks too, not statements.
 */
export interface Snak {
  /** The kind of the snak. */
  snaktype: SnakeType;
  /** The property the snak is about. */
  property: EntityId;
  /** The datatype of the property. */
  datatype?: PropertyDatatype;
  /** The value. It is absent when the snak type is not `value`. */
  datavalue?: DataValueContainer;
}

/**
 * A group of snaks, as a reference holds.
 *
 * A reference is a set of snaks that together say where a statement
 * came from.
 */
export interface SnakGroup {
  /** The snaks of the group, by property. */
  snaks?: Record<EntityId, Snak[]>;
}

/**
 * A statement: one claim of one property, with its qualifiers and its
 * references.
 */
export interface Statement {
  /** The identifier of the statement. */
  id?: EntityId;
  /** The value of the statement. */
  mainsnak?: Snak;
  /** The statements that narrow the value down. */
  qualifiers?: Record<EntityId, Snak[]>;
  /** The properties the qualifiers are shown in, in the order of the dump. */
  "qualifiers-order"?: EntityId[];
  /** Where the statement comes from. */
  references?: SnakGroup[];
  /** The kind of the statement. */
  type?: "statement";
  /** How good the statement is thought to be. */
  rank?: StatementRank;
}

/** The claims of an item, by property. */
export type Claims = Record<EntityId, Statement[]>;

/**
 * An item of the dump, as it is read.
 *
 * This is the raw shape. The fields that this panel does not use are
 * declared but empty, so that the type describes the dump rather than
 * the panel.
 */
export interface WikidataItem {
  /** The identifier of the entity, such as "Q90". */
  id: EntityId;
  /** The kind of the entity. */
  type: EntityType;
  /** The labels, by language. */
  labels?: Record<string, Term>;
  /** The descriptions, by language. */
  descriptions?: Record<string, Term>;
  /** The aliases, by language. Each language holds a list. */
  aliases?: Record<string, Term[]>;
  /** The claims, by property. */
  claims?: Claims;
  /** The sitelinks, by site. */
  sitelinks?: Record<string, { site: string; title: string }>;
  /** The datatype, for a property. */
  datatype?: PropertyDatatype;
}



/**
 * A row of the place list the extractor writes.
 *
 * The file holds one plain identifier per line, so a line is a JSON
 * string rather than an object.
 */
export type PlaceQidRow = EntityId;

/**
 * The properties the panel gives a name to.
 *
 * The dump carries no labels for the properties, so the ones the
 * panel shows by name are named here. Anything else is shown with its
 * property identifier alone.
 */
export const PROPERTY_LABELS: Readonly<Record<EntityId, string>> = {
  P18: "Image",
  P31: "Instance of",
  P106: "Occupation",
  P131: "Located in",
  P136: "Genre",
  P138: "Named after",
  P166: "Award received",
  P276: "Location",
  P361: "Part of",
  P571: "Inception",
  P577: "Publication date",
  P625: "Coordinate location",
  P915: "Filming location",
  P840: "Narrative location",
  P345: "IMDb ID",
  P1476: "Title",
  P50: "Author",
  P57: "Director",
  P58: "Screenwriter",
  P161: "Cast member",
  P175: "Performer",
  P463: "Member of",
  P527: "Has part",
  P279: "Subclass of",
  P17: "Country",
  P1082: "Population",
  P2044: "Elevation above sea level",
  P2046: "Area",
  P2043: "Length",
  P2124: "Day",
  P5710: "Time of discovery or invention",
  P793: "Significant event",
  P39: "Position held",
  P54: "Member of sports team",
  P735: "Given name",
  P734: "Family name",
  P170: "Creator",
  P112: "Founded by",
};
