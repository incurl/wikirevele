/**
 * The view model: what the components read.
 *
 * The dump is the raw layer and it is never mutated. Everything the
 * panel shows is derived here, into plain read-only objects, so that a
 * component cannot accidentally change the data it was given and that
 * the same item always renders the same way.
 */

import type {
  EntityId,
  PropertyDatatype,
  Snak,
  Statement,
  StatementRank,
  WikidataItem,
} from "./types";
import { PROPERTY_LABELS } from "./types";

/**
 * The languages the panel looks for, in order of preference.
 *
 * English comes first because the Paris exploration reads better with
 * the English label, and French is the fallback for the places that
 * have no English label.
 */
export const PREFERRED_LANGUAGES = ["en", "fr"] as const;

/** The kind of a language, for the badge on a term. */
export type TermLanguage = "primary" | "fallback" | "none";

/**
 * A term of an entity, resolved for display.
 */
export interface ResolvedTerm {
  /** The text of the term. */
  readonly text: string;
  /** The language of the term, empty when there is none. */
  readonly language: string;
  /** How the language was chosen. */
  readonly languageKind: TermLanguage;
}

/**
 * A claim of an item, resolved for display.
 *
 * A claim is one property with all of its statements. The statements
 * are kept in the order of the dump.
 */
export interface ResolvedClaim {
  /** The identifier of the property, such as "P131". */
  readonly propertyId: EntityId;
  /** The name of the property, when the panel knows one. */
  readonly propertyLabel: string;
  /** The statements of the property. */
  readonly statements: readonly ResolvedStatement[];
  /** The number of statements. */
  readonly count: number;
}

/**
 * One statement, resolved for display.
 */
export interface ResolvedStatement {
  /** The identifier of the statement. */
  readonly id: string;
  /** The rank of the statement. */
  readonly rank: StatementRank;
  /** The value of the statement, absent when it has none. */
  readonly value: ResolvedValue | null;
  /** The qualifiers of the statement. */
  readonly qualifiers: readonly ResolvedQualifier[];
  /** The references of the statement. */
  readonly references: readonly ResolvedReference[];
}

/**
 * A qualifier of a statement, resolved for display.
 */
export interface ResolvedQualifier {
  /** The identifier of the property. */
  readonly propertyId: EntityId;
  /** The name of the property, when the panel knows one. */
  readonly propertyLabel: string;
  /** The value of the qualifier. */
  readonly value: ResolvedValue;
}

/**
 * A reference of a statement, resolved for display.
 */
export interface ResolvedReference {
  /** The properties the reference cites. */
  readonly entries: readonly ResolvedQualifier[];
}

/**
 * A value of a statement, resolved for display.
 *
 * The kind says which renderer to use. The payload holds the parts the
 * renderer needs, so that a renderer never has to look at the raw
 * value again.
 */
export type ResolvedValue =
  | { readonly kind: "entity"; readonly id: EntityId; readonly entityType: "item" | "property" }
  | { readonly kind: "text"; readonly text: string }
  | { readonly kind: "url"; readonly url: string }
  | { readonly kind: "external-id"; readonly id: string; readonly url: string | null }
  | { readonly kind: "monolingual"; readonly text: string; readonly language: string }
  | { readonly kind: "time"; readonly time: TimeParts }
  | { readonly kind: "quantity"; readonly amount: string; readonly unit: string }
  | { readonly kind: "coordinate"; readonly latitude: number; readonly longitude: number; readonly globe: string | null }
  | { readonly kind: "no-value" }
  | { readonly kind: "some-value" };

/**
 * A time value, broken into its parts.
 *
 * The precision decides how much of the date is shown: a year is
 * shown as a year, and a day is shown as a day.
 */
export interface TimeParts {
  /** The year, negative for a date before the common era. */
  readonly year: number;
  /** The month, 1 to 12, or 0 when the precision is a year. */
  readonly month: number;
  /** The day of the month, or 0 when the precision is below a day. */
  readonly day: number;
  /** The precision: 0 is a year, 11 is a second. */
  readonly precision: number;
  /** Non-zero when the date is before the earliest known date. */
  readonly before: number;
  /** Non-zero when the date is after the latest known date. */
  readonly after: number;
}

/**
 * The whole item, resolved for display.
 */
export interface EntityView {
  /** The identifier of the entity, such as "Q90". */
  readonly id: EntityId;
  /** The label to show, with the language it was taken from. */
  readonly label: ResolvedTerm;
  /** The description to show, with the language it was taken from. */
  readonly description: ResolvedTerm;
  /** The aliases, in the preferred languages. */
  readonly aliases: readonly ResolvedTerm[];
  /** The claims of the item, in a stable order. */
  readonly claims: readonly ResolvedClaim[];
  /** The claims, indexed by property, for the filters. */
  readonly claimsByProperty: ReadonlyMap<EntityId, readonly ResolvedStatement[]>;
  /** The identifier of the image of the item, when it has one. */
  readonly imageId: EntityId | null;
  /** The number of claims. */
  readonly claimCount: number;
}

/**
 * Resolves the term of an entity in the preferred languages.
 *
 * The first language of `PREFERRED_LANGUAGES` that the entity has wins.
 * The result says which language it was, so that the card can show the
 * fallback in a quieter way.
 */
export function resolveTerm(
  terms: Record<string, { language: string; value: string }> | undefined,
): ResolvedTerm {
  if (!terms) {
    return { text: "", language: "", languageKind: "none" };
  }

  for (const language of PREFERRED_LANGUAGES) {
    const term = terms[language];
    if (term?.value) {
      return {
        text: term.value,
        language,
        languageKind: language === PREFERRED_LANGUAGES[0] ? "primary" : "fallback",
      };
    }
  }

  return { text: "", language: "", languageKind: "none" };
}

/**
 * Resolves the aliases of an entity in the preferred languages.
 */
export function resolveAliases(
  aliases: Record<string, { language: string; value: string }[]> | undefined,
): ResolvedTerm[] {
  if (!aliases) {
    return [];
  }

  const out: ResolvedTerm[] = [];
  for (const language of PREFERRED_LANGUAGES) {
    for (const alias of aliases[language] ?? []) {
      if (alias.value) {
        out.push({
          text: alias.value,
          language,
          languageKind: language === PREFERRED_LANGUAGES[0] ? "primary" : "fallback",
        });
      }
    }
  }
  return out;
}

/**
 * The name of a property, when the panel knows one.
 *
 * The dump carries no labels for the properties, so the panel names the
 * ones it shows often and shows the identifier for the rest.
 */
export function propertyLabel(propertyId: EntityId): string {
  return PROPERTY_LABELS[propertyId] ?? propertyId;
}

/**
 * Breaks a time value into its parts.
 *
 * The `time` field of the dump is a string with a leading sign and a
 * year that may be padded to more than four digits, so it is read
 * with a regular expression rather than by slicing.
 */
export function parseTime(value: {
  time: string;
  precision: number;
  before?: number;
  after?: number;
}): TimeParts {
  const match = /^([+-])(\d{1,})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})Z?$/.exec(value.time);

  if (!match) {
    return { year: 0, month: 0, day: 0, precision: value.precision, before: value.before ?? 0, after: value.after ?? 0 };
  }

  const [, sign, yearText, monthText, dayText] = match;
  const year = Number(yearText) * (sign === "-" ? -1 : 1);

  // The precision says how much of the date is known: below a day the
  // month and the day are not meaningful.
  const precision = value.precision;
  const month = precision >= 1 ? Number(monthText) : 0;
  const day = precision >= 2 ? Number(dayText) : 0;

  return {
    year,
    month,
    day,
    precision,
    before: value.before ?? 0,
    after: value.after ?? 0,
  };
}

/**
 * The URL of the entity page of a Wikidata identifier.
 */
export function entityUrl(id: EntityId): string {
  return `https://www.wikidata.org/wiki/${id}`;
}

/**
 * The URL of the file of a Wikimedia Commons identifier.
 */
export function commonsUrl(fileName: string): string {
  return `https://commons.wikimedia.org/wiki/Special:Redirect/file/${encodeURIComponent(fileName)}`;
}

/**
 * The resolver of an external identifier.
 *
 * The panel knows the resolvers of the identifiers it shows. An
 * identifier it does not know has no resolver, and is shown as plain
 * text.
 */
const EXTERNAL_ID_RESOLVERS: Readonly<Record<EntityId, string>> = {
  P213: "https://www.isni.org/",
  P214: "https://viaf.org/viaf/",
  P227: "https://d-nb.info/gnd/",
  P244: "https://id.loc.gov/authorities/names/",
  P268: "https://www.bnf.fr/",
  P269: "https://www.idref.fr/",
  P345: "https://www.imdb.com/title/",
  P409: "https://www.imdb.com/name/",
  P434: "https://musicbrainz.org/",
  P646: "https://www.freebase.com/",
  P729: "https://www.29a.org/",
  P950: "https://www.biolib.cz/",
  P1207: "https://www.youtube.com/",
};

/**
 * The URL that resolves an external identifier, when the panel knows
 * one.
 */
export function externalIdUrl(propertyId: EntityId, id: string): string | null {
  const base = EXTERNAL_ID_RESOLVERS[propertyId];
  if (!base) {
    return null;
  }
  return base + encodeURIComponent(id);
}

/**
 * Resolves the value of a snak for display.
 *
 * The `datatype` of the snak says how the value should be read, and the
 * `type` of the value says what it holds. A snak without a value is
 * either a "no value" or a "some value", and the panel says so rather
 * than showing nothing.
 */
export function resolveValue(snak: Snak): ResolvedValue {
  if (snak.snaktype === "novalue") {
    return { kind: "no-value" };
  }
  if (snak.snaktype === "somevalue") {
    return { kind: "some-value" };
  }

  const container = snak.datavalue;
  if (!container) {
    return { kind: "no-value" };
  }

  const datatype: PropertyDatatype | undefined = snak.datatype;

  // The kind of the value is the `type` of the container, next to the
  // value itself: `"datavalue": { "value": { ... }, "type": "..." }`.
  switch (container.type) {
    case "wikibase-entityid":
      return {
        kind: "entity",
        id: container.value.id,
        entityType: container.value["entity-type"],
      };
    case "monolingualtext":
      return { kind: "monolingual", text: container.value.text, language: container.value.language };
    case "time":
      return { kind: "time", time: parseTime(container.value) };
    case "quantity":
      return { kind: "quantity", amount: container.value.amount, unit: container.value.unit };
    case "globecoordinate":
      return {
        kind: "coordinate",
        latitude: container.value.latitude,
        longitude: container.value.longitude,
        globe: container.value.globe ?? null,
      };
    case "external-id":
      return {
        kind: "external-id",
        id: container.value,
        url: externalIdUrl(snak.property, container.value),
      };
    case "string":
      // The datatype tells a plain string from a URL: a URL property
      // holds a string, and is a link.
      if (datatype === "url") {
        return { kind: "url", url: container.value };
      }
      return { kind: "text", text: container.value };
  }
}

/**
 * Resolves one statement for display.
 */
export function resolveStatement(statement: Statement, index: number): ResolvedStatement {
  const mainsnak = statement.mainsnak;

  const qualifiers: ResolvedQualifier[] = [];
  if (mainsnak && statement.qualifiers) {
    // The order of the qualifiers is the order of the dump, so that
    // two runs render the same statement the same way.
    const order = statement["qualifiers-order"] ?? Object.keys(statement.qualifiers);
    for (const propertyId of order) {
      for (const snak of statement.qualifiers[propertyId] ?? []) {
        qualifiers.push({
          propertyId,
          propertyLabel: propertyLabel(propertyId),
          value: resolveValue(snak),
        });
      }
    }
  }

  const references: ResolvedReference[] = [];
  for (const reference of statement.references ?? []) {
    const entries: ResolvedQualifier[] = [];
    for (const [propertyId, snaks] of Object.entries(reference.snaks ?? {})) {
      for (const snak of snaks) {
        entries.push({
          propertyId,
          propertyLabel: propertyLabel(propertyId),
          value: resolveValue(snak),
        });
      }
    }
    references.push({ entries });
  }

  return {
    id: statement.id ?? `statement-${index}`,
    rank: statement.rank ?? "normal",
    value: mainsnak ? resolveValue(mainsnak) : null,
    qualifiers,
    references,
  };
}

/**
 * Builds the view model of an item.
 *
 * The item is read, never changed: every value in the result is
 * derived from it.
 */
export function toEntityView(item: WikidataItem): EntityView {
  const claims = item.claims ?? {};

  const resolvedClaims: ResolvedClaim[] = [];
  const claimsByProperty = new Map<EntityId, ResolvedStatement[]>();

  for (const propertyId of Object.keys(claims).sort()) {
    const statements = claims[propertyId] ?? [];
    if (statements.length === 0) {
      continue;
    }

    const resolved = statements.map((statement, index) => resolveStatement(statement, index));
    claimsByProperty.set(propertyId, resolved);
    resolvedClaims.push({
      propertyId,
      propertyLabel: propertyLabel(propertyId),
      statements: resolved,
      count: resolved.length,
    });
  }

  const imageStatements = claimsByProperty.get("P18");
  const imageId =
    imageStatements && imageStatements[0]?.value?.kind === "entity" ? imageStatements[0].value.id : null;

  return {
    id: item.id,
    label: resolveTerm(item.labels),
    description: resolveTerm(item.descriptions),
    aliases: resolveAliases(item.aliases),
    claims: resolvedClaims,
    claimsByProperty,
    imageId,
    claimCount: resolvedClaims.length,
  };
}

/**
 * The values of a property, as the identifiers they point at.
 *
 * The filters use this: a filter on "located in" asks whether the list
 * holds a given identifier.
 */
export function claimEntityIds(
  view: EntityView,
  propertyId: EntityId,
): readonly EntityId[] {
  const statements = view.claimsByProperty.get(propertyId) ?? [];
  const out: EntityId[] = [];
  for (const statement of statements) {
    if (statement.value?.kind === "entity") {
      out.push(statement.value.id);
    }
  }
  return out;
}

/**
 * The claims of an item that point at another item, for the graph.
 *
 * The graph shows the neighbourhood of an item, so only the claims
 * that point at an entity are of interest.
 */
export interface GraphEdge {
  readonly from: EntityId;
  readonly to: EntityId;
  readonly propertyId: EntityId;
}

export interface GraphData {
  readonly nodes: readonly { id: EntityId; label: string }[];
  readonly edges: readonly GraphEdge[];
}

/**
 * Builds the graph of an item.
 *
 * The node count is capped: a dump item can hold thousands of claims,
 * and a graph of that size is neither readable nor fast.
 */
export function toGraphData(view: EntityView, maxNodes = 40): GraphData {
  const nodes = new Map<EntityId, string>();
  const edges: GraphEdge[] = [];

  nodes.set(view.id, view.label.text || view.id);

  for (const claim of view.claims) {
    for (const statement of claim.statements) {
      if (statement.value?.kind !== "entity") {
        continue;
      }
      if (nodes.size >= maxNodes && !nodes.has(statement.value.id)) {
        continue;
      }
      nodes.set(statement.value.id, statement.value.id);
      edges.push({ from: view.id, to: statement.value.id, propertyId: claim.propertyId });
    }
  }

  return { nodes: [...nodes].map(([id, label]) => ({ id, label })), edges };
}
