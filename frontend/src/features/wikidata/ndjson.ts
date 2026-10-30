/**
 * The source that holds the imported records in memory.
 *
 * The records come from the database, where the importer put them, and
 * the panel filters them in memory. The Paris dataset is small enough
 * for that to be fine.
 *
 * The file is called NDJSON because that is the shape the importer
 * reads, but the panel never parses one: the backend does the reading,
 * and the panel receives the records.
 */

import type { EntityId, WikidataItem } from "./types";
import type { WikidataSource } from "./source";

/**
 * A source that holds the items in memory.
 */
export class InMemorySource implements WikidataSource {
  /** The name of the source, shown in the interface. */
  readonly name: string;

  /** The items of the source, by identifier. */
  private readonly items: Map<EntityId, WikidataItem>;

  /**
   * Creates a source that holds the given items.
   */
  constructor(name: string, items: WikidataItem[]) {
    this.name = name;
    this.items = new Map(items.map((item) => [item.id, item]));
  }

  /**
   * Returns every item of the source.
   */
  async listItems(): Promise<WikidataItem[]> {
    return [...this.items.values()];
  }

  /**
   * Returns one item, by its identifier.
   */
  async getItem(id: EntityId): Promise<WikidataItem | null> {
    return this.items.get(id) ?? null;
  }

  /**
   * The number of items of the source.
   */
  get size(): number {
    return this.items.size;
  }
}
