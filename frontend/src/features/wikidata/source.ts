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
