/**
 * The Refine data provider for the Wikidata panel.
 *
 * The panel is not backed by a REST API: the items come from a local
 * subset, either the NDJSON files the extractor writes or the
 * PocketBase collections that hold a copy of them. Refine does not
 * care: a data provider is a plain object of async functions, so the
 * same contract is filled here.
 *
 * The filters are the interesting part. A Wikidata item has no
 * columns: its properties are claims, so a filter on a property is a
 * filter on a claim. The provider understands a filter whose field is
 * `claims.P131`, which is the property, and applies it to the values
 * the claims hold.
 */

import type {
  BaseKey,
  BaseRecord,
  CrudFilter,
  CrudSort,
  LogicalFilter,
  DataProvider,
  GetListParams,
  GetListResponse,
  GetManyParams,
  GetManyResponse,
  GetOneParams,
  GetOneResponse,
} from "@refinedev/core";

import type { EntityId, WikidataItem } from "./types";
import type { WikidataSource } from "./source";
import { toEntityView } from "./view";

/**
 * The name of the resource the panel shows.
 */
export const RESOURCE = "wikidata_item";

/**
 * The record the panel works with.
 *
 * Refine needs an `id` on every record. The identifier of a Wikidata
 * item is its QID, so the record carries it and the item itself.
 */
export interface WikidataRecord extends BaseRecord {
  /** The identifier of the item, which is its QID. */
  id: EntityId;
  /** The item, as it is in the dump. */
  item: WikidataItem;
}

/**
 * The prefix of a filter on a property.
 *
 * A filter whose field is `claims.P131` filters on the claims of the
 * property `P131`.
 */
const CLAIM_FIELD_PREFIX = "claims.";

/**
 * Whether a filter is a simple one.
 *
 * A `CrudFilter` is a union: a conditional filter groups other filters
 * and has no field of its own. The guard narrows the union, so that
 * the rest of the panel does not have to.
 */
export function isLogicalFilter(filter: CrudFilter): filter is LogicalFilter {
  return filter.operator !== "and" && filter.operator !== "or";
}

/**
 * The field of a filter, when the filter is a simple one.
 */
export function filterField(filter: CrudFilter): string | null {
  return isLogicalFilter(filter) ? filter.field : null;
}

/**
 * The value of a filter, when the filter is a simple one.
 */
export function filterValue(filter: CrudFilter): unknown {
  return isLogicalFilter(filter) ? filter.value : null;
}

/**
 * Reads the property a filter is about, when it is about a claim.
 */
export function claimPropertyOf(field: string): EntityId | null {
  if (!field.startsWith(CLAIM_FIELD_PREFIX)) {
    return null;
  }
  const property = field.slice(CLAIM_FIELD_PREFIX.length);
  return property.length > 0 ? property : null;
}

/**
 * The values of a property of an item, as plain strings.
 *
 * The values are the identifiers of the entities the property points
 * at, and the text of the other values. A filter on a property is a
 * filter on this list.
 */
export function claimValues(item: WikidataItem, property: EntityId): string[] {
  const statements = item.claims?.[property] ?? [];
  const out: string[] = [];

  for (const statement of statements) {
    const snak = statement.mainsnak;
    if (!snak || snak.snaktype !== "value" || !snak.datavalue) {
      continue;
    }

    // The kind of the value is the `type` of the container, next to the
    // value itself: `"datavalue": { "value": { ... }, "type": "..." }`.
    const container = snak.datavalue;
    switch (container.type) {
      case "wikibase-entityid":
        out.push(container.value.id);
        break;
      case "string":
      case "external-id":
        out.push(container.value);
        break;
      case "monolingualtext":
        out.push(container.value.text);
        break;
      default:
        break;
    }
  }

  return out;
}

/**
 * Applies one filter to an item.
 */
export function matchesFilter(item: WikidataItem, filter: CrudFilter): boolean {
  // A conditional filter is a group: every filter of the group must
  // match, and the groups are combined with the operator.
  if (filter.operator === "and" || filter.operator === "or") {
    const group = filter.value;
    if (!Array.isArray(group) || group.length === 0) {
      return true;
    }
    const results = group.map((inner) => matchesFilter(item, inner));
    return filter.operator === "and" ? results.every(Boolean) : results.some(Boolean);
  }

  const field = filterField(filter);
  if (field === null) {
    return true;
  }

  const property = claimPropertyOf(field);

  // A filter that is not about a claim is on the fields of the item
  // itself, which are the identifier and the labels.
  if (!property) {
    return matchesItemField(item, filter);
  }

  const values = claimValues(item, property);
  return matchesValues(values, filter);
}

/**
 * Applies a filter on the fields of the item itself.
 */
function matchesItemField(item: WikidataItem, filter: CrudFilter): boolean {
  if (filter.operator === "and" || filter.operator === "or") {
    return true;
  }

  const field = filterField(filter);
  if (field === null) {
    return true;
  }

  const term = item.labels?.[field];
  const values = term ? [term.value] : [];

  if (field === "id") {
    values[0] = item.id;
  }

  return matchesValues(values, filter);
}

/**
 * Applies a filter to the values of a property.
 *
 * The operators are the ones Refine defines. The two that matter most
 * for a property are `eq`, which asks whether one of the values is the
 * one, and `contains`, which asks whether the text of one of the values
 * holds the text.
 */
function matchesValues(values: string[], filter: CrudFilter): boolean {
  if (filter.operator === "and" || filter.operator === "or") {
    return true;
  }

  const wanted = filter.value;

  switch (filter.operator) {
    case "eq":
      return values.some((value) => value === wanted);
    case "ne":
      return !values.some((value) => value === wanted);
    case "in":
      return Array.isArray(wanted) && values.some((value) => wanted.includes(value));
    case "nin":
      return Array.isArray(wanted) && !values.some((value) => wanted.includes(value));
    case "contains":
      return values.some((value) => value.toLowerCase().includes(String(wanted).toLowerCase()));
    case "ncontains":
      return !values.some((value) => value.toLowerCase().includes(String(wanted).toLowerCase()));
    case "startswith":
      return values.some((value) => value.toLowerCase().startsWith(String(wanted).toLowerCase()));
    case "endswith":
      return values.some((value) => value.toLowerCase().endsWith(String(wanted).toLowerCase()));
    case "null":
      return values.length === 0;
    case "nnull":
      return values.length > 0;
    case "gt":
    case "gte":
    case "lt":
    case "lte": {
      // A comparison on the values of a property only makes sense on
      // numbers, so the values that are not numbers are skipped.
      const target = Number(wanted);
      if (Number.isNaN(target)) {
        return false;
      }
      return values.some((value) => {
        const number = Number(value);
        if (Number.isNaN(number)) {
          return false;
        }
        switch (filter.operator) {
          case "gt":
            return number > target;
          case "gte":
            return number >= target;
          case "lt":
            return number < target;
          default:
            return number <= target;
        }
      });
    }
    default:
      // An operator the panel does not implement does not hide the
      // item: it leaves it in.
      return true;
  }
}

/**
 * Applies the sorts to a list of items.
 */
function sortItems(items: WikidataItem[], sorters: CrudSort[] | undefined): WikidataItem[] {
  if (!sorters || sorters.length === 0) {
    return items;
  }

  const sorted = [...items];
  sorted.sort((left, right) => {
    for (const sorter of sorters) {
      const a = sortableValue(left, sorter.field);
      const b = sortableValue(right, sorter.field);
      if (a === b) {
        continue;
      }
      const order = a < b ? -1 : 1;
      return sorter.order === "desc" ? -order : order;
    }
    return 0;
  });
  return sorted;
}

/**
 * The value of an item that a sort is about.
 */
function sortableValue(item: WikidataItem, field: string): string {
  const property = claimPropertyOf(field);
  if (property) {
    return claimValues(item, property)[0] ?? "";
  }
  if (field === "id") {
    return item.id;
  }
  return item.labels?.[field]?.value ?? "";
}

/**
 * Creates the Refine data provider over a source.
 *
 * The source is a view of the imported records. The provider reads it
 * on every call, so that an import is visible without a reload: the
 * dataset of a whole city is small enough for that to be fine.
 */
export function createWikidataProvider(source: WikidataSource): DataProvider {
  /**
   * Reads the items of the source.
   */
  const items = async (): Promise<WikidataItem[]> => source.listItems();

  return {
    getApiUrl: () => "",

    getList: async <TData extends BaseRecord = BaseRecord>(
      params: GetListParams,
    ): Promise<GetListResponse<TData>> => {
      const all = await items();

      const filters = params.filters ?? [];
      const filtered = filters.length === 0 ? all : all.filter((item) => filters.every((f) => matchesFilter(item, f)));

      const sorted = sortItems(filtered, params.sorters);

      const current = params.pagination?.currentPage ?? 1;
      const size = params.pagination?.pageSize ?? 25;
      const start = (current - 1) * size;
      const page = sorted.slice(start, start + size);

      return {
        data: page.map((item) => toRecord(item)) as unknown as TData[],
        total: sorted.length,
      };
    },

    getOne: async <TData extends BaseRecord = BaseRecord>(
      params: GetOneParams,
    ): Promise<GetOneResponse<TData>> => {
      const item = await source.getItem(String(params.id));
      if (!item) {
        throw new Error(`wikidata: the item ${String(params.id)} is not in the subset`);
      }
      return { data: toRecord(item) as unknown as TData };
    },

    getMany: async <TData extends BaseRecord = BaseRecord>(
      params: GetManyParams,
    ): Promise<GetManyResponse<TData>> => {
      const all = await items();
      const wanted = new Set(params.ids.map(String));
      const found = all.filter((item) => wanted.has(item.id));
      return { data: found.map((item) => toRecord(item)) as unknown as TData[] };
    },

    // The panel is read-only: the items are the dump's, and the dump is
    // the source of truth. A mutation is refused loudly rather than
    // silently doing nothing.
    create: async () => {
      throw new Error("wikidata: the panel is read-only, an item cannot be created");
    },
    update: async () => {
      throw new Error("wikidata: the panel is read-only, an item cannot be updated");
    },
    deleteOne: async () => {
      throw new Error("wikidata: the panel is read-only, an item cannot be deleted");
    },
  };
}

/**
 * Turns an item into the record Refine works with.
 */
export function toRecord(item: WikidataItem): WikidataRecord {
  return { id: item.id, item };
}

/**
 * Reads the item back out of a record.
 */
export function fromRecord(record: BaseRecord | undefined): WikidataItem | null {
  if (!record) {
    return null;
  }
  const item = (record as WikidataRecord).item;
  return item ?? null;
}

/**
 * Reads the view model out of a record.
 */
export function viewOf(record: BaseRecord | undefined): ReturnType<typeof toEntityView> | null {
  const item = fromRecord(record);
  return item ? toEntityView(item) : null;
}

/**
 * The key of an identifier, for the parts of Refine that want one.
 */
export function toBaseKey(id: EntityId): BaseKey {
  return id;
}
