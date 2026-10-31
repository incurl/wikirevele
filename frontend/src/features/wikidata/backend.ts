/**
 * The source that reads the imported dataset from the backend.
 *
 * The panel does not read the files itself: the backend reads them
 * once, during the import, and the panel then works on the records of
 * the database. This is the rule of the repository, and it is also
 * what makes the panel show the same data after a restart.
 */

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import * as api from "../../bindings";
import type { WikidataItems } from "../../bindings";
import type { EntityId, WikidataItem } from "./types";
import type { Page, PageRequest, PaginatedSource } from "./source";
import type { EntityView } from "./view";
import { InMemorySource } from "./ndjson";

/**
 * The source over the records of the database.
 */
export function sourceFromItems(items: WikidataItems): InMemorySource {
  const records = items.items.map((item) => ({
    id: item.qid,
    type: "item",
    labels: Object.fromEntries(
      Object.entries(item.labels ?? {}).map(([language, value]) => [
        language,
        { language, value },
      ]),
    ),
    descriptions: Object.fromEntries(
      Object.entries(item.descriptions ?? {}).map(([language, value]) => [
        language,
        { language, value },
      ]),
    ),
    claims: (item.claims ?? {}) as WikidataItem["claims"],
  })) as unknown as WikidataItem[];

  return new InMemorySource("pocketbase", records);
}

/**
 * The source that asks the backend for a page at a time.
 *
 * The filtering, the sorting and the counting happen in the database,
 * so the gallery shows a dataset of any size: it never holds more than
 * the page it renders.
 */
class BackendSource implements PaginatedSource {
  readonly name = "pocketbase";

  // The whole dataset is never read: the gallery asks for a page, and
  // these two methods are only here because the interface has them.
  async listItems(): Promise<WikidataItem[]> {
    const page = await api.getWikidataItemsPage({ limit: PAGE_SIZE, offset: 0, sortField: "qid", sortDescending: false, filters: [] });
    return itemsOf(page.items);
  }

  async getItem(id: EntityId): Promise<WikidataItem | null> {
    const page = await api.getWikidataItems(1, 0);
    const match = page.items.find((item) => item.qid === id);
    return match ? (itemsOf([match])[0] ?? null) : null;
  }

  async listPage(request: PageRequest): Promise<Page> {
    const page = await api.getWikidataItemsPage({
      limit: request.limit,
      offset: request.offset,
      sortField: request.sortField,
      sortDescending: request.sortDescending,
      filters: request.filters.map((filter) => ({ property: filter.property, value: filter.value })),
    });

    return { items: itemsOf(page.items), total: page.total };
  }
}

/**
 * Turns the records the backend sent into the items the panel reads.
 */
function itemsOf(items: readonly WikidataItems["items"][number][]): WikidataItem[] {
  return items.map(
    (item) =>
      ({
        id: item.qid,
        type: "item",
        labels: Object.fromEntries(
          Object.entries(item.labels ?? {}).map(([language, value]) => [language, { language, value }]),
        ),
        descriptions: Object.fromEntries(
          Object.entries(item.descriptions ?? {}).map(([language, value]) => [language, { language, value }]),
        ),
        claims: (item.claims ?? {}) as WikidataItem["claims"],
      }) as unknown as WikidataItem,
  );
}

/**
 * The state of the dataset, while it is being read.
 */
export interface DatasetState {
  /** The source, once the dataset is known. */
  readonly source: PaginatedSource | null;
  /** The names of the properties, by identifier. */
  readonly propertyLabels: Record<string, string>;
  /** The number of items in the dataset. */
  readonly total: number;
  /** Whether the dataset is being read. */
  readonly loading: boolean;
  /** The error, when the dataset could not be read. */
  readonly error: Error | null;
}

/**
 * The number of items the panel asks for in one read.
 *
 * The dataset of a whole city is large, so the panel reads it in
 * pages rather than all at once.
 */
export const PAGE_SIZE = 200;

/**
 * The key of the dataset query.
 *
 * The import panel invalidates it when an import has written records,
 * so that the list shows them without a reload.
 */
export const DATASET_QUERY_KEY = ["wikidata", "items"];

/**
 * Reads the dataset and keeps it.
 *
 * The dataset is read through React Query, so that an import can
 * refresh it: the panel shows what is in the database, and the
 * database changes when a file is imported.
 */
export function useWikidataDataset(): DatasetState {
  const query = useQuery({
    queryKey: DATASET_QUERY_KEY,
    queryFn: () => api.getWikidataItems(PAGE_SIZE, 0),
  });

  const items = query.data;

  // The source is built again whenever the records change, so that the
  // provider reads the new ones after an import.
  //
  // It filters and pages in the backend, so the gallery never holds
  // more than the page it renders.
  const source = useMemo(
    () => (items ? new BackendSource() : null),
    [items],
  );

  return {
    source,
    propertyLabels: items?.properties ?? {},
    total: items?.total ?? 0,
    loading: query.isLoading,
    error: query.error instanceof Error ? query.error : null,
  };
}

/**
 * The places of an item, read from its claims.
 *
 * The relationships of the old extractor are gone: the panel reads the
 * places from the claims of the item itself, so that the graph and the
 * cards agree with what is in the database.
 *
 * P915 is the filming location and P840 the narrative one.
 */
export function placesOf(view: EntityView): readonly PlaceLink[] {
  const out: PlaceLink[] = [];

  for (const [property, kind] of [
    ["P915", "filming"],
    ["P840", "narrative"],
  ] as const) {
    for (const statement of view.claimsByProperty.get(property) ?? []) {
      const value = statement.value;
      if (!value || value.kind !== "entity") {
        continue;
      }
      out.push({ placeQid: value.id, propertyKind: kind });
    }
  }

  return out;
}

/**
 * A place an item links to.
 */
export interface PlaceLink {
  /** The identifier of the place. */
  readonly placeQid: string;
  /** Whether the place is a filming location or a narrative one. */
  readonly propertyKind: "filming" | "narrative";
}

/**
 * The identifiers of the items, for the tests.
 */
export type { EntityId };
