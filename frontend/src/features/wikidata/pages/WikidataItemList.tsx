/**
 * The list page of the panel.
 *
 * It shows a grid of the cards of the items, with the filter bar above
 * it. The page reads the items through Refine, so the pagination, the
 * filters and the loading state are the ones of the framework.
 */

import { useMemo, useState } from "react";
import { useList } from "@refinedev/core";
import { Loader2 } from "lucide-react";

import { WikidataEntityCard } from "../components/WikidataEntityCard";
import { FilterBar } from "../components/FilterBar";
import { RESOURCE } from "../provider";
import { viewOf } from "../provider";
import { placesOf } from "../backend";

/**
 * The props of the list page.
 */
export interface WikidataItemListProps {
  /** Whether the graph is shown in the cards. */
  readonly showGraph?: boolean;
}

/**
 * The list page.
 */
export function WikidataItemList({ showGraph = false }: WikidataItemListProps) {
  const [filters, setFilters] = useState<import("@refinedev/core").CrudFilter[]>([]);

  const { result, query } = useList({
    resource: RESOURCE,
    pagination: { currentPage: 1, pageSize: 24 },
    filters,
  });

  const { data, total } = result;

  const cards = useMemo(
    () =>
      (data ?? []).map((record) => {
        const view = viewOf(record);
        return {
          record,
          view,
          // The places come from the claims of the item itself, so
          // that the cards and the graph agree with the database.
          places: view ? placesOf(view) : [],
        };
      }),
    [data],
  );

  if (query.isLoading) {
    return (
      <div className="flex items-center justify-center gap-2 p-8 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        Loading the items…
      </div>
    );
  }

  if (query.isError) {
    return (
      <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-4 text-sm">
        <p className="font-medium">The items could not be loaded.</p>
        <p className="mt-1 text-muted-foreground">{query.error?.message}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <FilterBar filters={filters} onChange={setFilters} total={total} />

      {cards.length === 0 ? (
        <p className="rounded-lg border border-border bg-card p-6 text-center text-sm text-muted-foreground">
          No item matches the filters.
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {cards.map(({ record, view, places }) =>
            view ? (
              <WikidataEntityCard
                key={String(record.id)}
                item={view}
                places={places}
                showGraph={showGraph}
              />
            ) : null,
          )}
        </div>
      )}
    </div>
  );
}
