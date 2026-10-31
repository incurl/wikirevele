/**
 * Where the panel gets its items.
 *
 * The panel works on the records of the database, where the importer
 * put them. The source is the boundary between that and the rest of
 * the panel: the provider asks it for the items, and the components
 * never know where the items came from.
 *
 * The interface is deliberately small. A source answers two questions:
 * which items exist, and what one item looks like.
 */

import type { EntityId, WikidataItem } from "./types";

/**
 * A read-only view of the imported dataset.
 */
export interface WikidataSource {
  /**
   * The name of the source, shown in the interface.
   */
  readonly name: string;

  /**
   * Returns every item of the dataset.
   *
   * A source that holds a lot of items may return a subset, and the
   * provider filters it in memory. The Paris dataset is small enough
   * for that to be fine.
   */
  listItems(): Promise<WikidataItem[]>;

  /**
   * Returns one item, by its identifier.
   *
   * It returns null when the dataset does not hold the item.
   */
  getItem(id: EntityId): Promise<WikidataItem | null>;
}

/**
 * A filter on the value of one property.
 *
 * An item has no columns: its properties are claims, so a filter on a
 * property is a filter on a claim.
 */
export interface ClaimFilterRequest {
  /** The property of the claim, such as "P131". */
  readonly property: string;
  /** The value the claim must hold, such as "Q90". */
  readonly value: string;
}

/**
 * A request for one page of a dataset.
 */
export interface PageRequest {
  /** The number of items to return. */
  readonly limit: number;
  /** The number of items to skip. */
  readonly offset: number;
  /** The field to sort on. */
  readonly sortField: string;
  /** Whether the order is reversed. */
  readonly sortDescending: boolean;
  /** The claims the items must hold. */
  readonly filters: readonly ClaimFilterRequest[];
}

/**
 * One page of a dataset.
 */
export interface Page {
  /** The items of the page. */
  readonly items: readonly WikidataItem[];
  /** The number of items that match the filters. */
  readonly total: number;
}

/**
 * A source that filters and pages in the backend.
 *
 * The gallery shows a dataset of a whole city, so the filtering, the
 * sorting and the counting have to happen where the data is. A source
 * that can do it says so by answering `listPage`.
 *
 * The in-memory source does not: it holds a subset that is small
 * enough to filter in the page, and it keeps working as it did.
 */
export interface PaginatedSource extends WikidataSource {
  /** Returns one page of the dataset. */
  listPage(request: PageRequest): Promise<Page>;
}

/**
 * Tells whether a source can filter and page in the backend.
 */
export function isPaginated(source: WikidataSource): source is PaginatedSource {
  return typeof (source as PaginatedSource).listPage === "function";
}

/**
 * A source that holds nothing.
 *
 * It is what the panel uses before the backend has answered, and what
 * the tests use. It never fails: it simply has no items.
 */
export const EMPTY_SOURCE: WikidataSource = {
  name: "none",
  listItems: async () => [],
  getItem: async () => null,
};
