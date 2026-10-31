/**
 * The list page of the panel: the card gallery of the workarea.
 *
 * The division of labour is deliberate:
 *
 * - **Refine's `useTable` owns the data.** The filtering, the sorting
 *   and the pagination are its state, and they are sent to the backend,
 *   which answers one page at a time. Nothing here filters a list in
 *   the page.
 * - **TanStack Table owns the shape of the rows.** It is given the
 *   records Refine returned and nothing else: it sorts and pages
 *   nothing, because the server already did. What it does own is the
 *   identity of the rows, which is the QID of the item, so that a card
 *   keeps its place when the page changes.
 * - **TanStack Virtual owns what is on screen.** It renders the cards
 *   that are in the viewport, so a page of a thousand cards costs the
 *   same as a page of twenty. The grid is made of the library's
 *   lanes, so the library lays the cards out and measures them.
 *
 * The page holds the query and the selection. The drawing is the work
 * of `GalleryGrid`, and the cards themselves are the ones of the
 * panel, and they are not aware of any of it.
 */

import { useCallback, useMemo, useState } from "react";
import { useTable } from "@refinedev/core";
import { Loader2 } from "lucide-react";

import { GalleryGrid } from "../components/GalleryGrid";
import { FilterBar } from "../components/FilterBar";
import { RESOURCE } from "../provider";
import type { WikidataRecord } from "../provider";
import {
  EMPTY_SELECTION,
  clear,
  selectAll,
  summarize,
  toggle,
  type Selection,
} from "../selection";
import { pageSummary } from "../gallery";

/**
 * The props of the list page.
 */
export interface WikidataItemListProps {
  /** Whether the graph is shown in the cards. */
  readonly showGraph?: boolean;
}

/**
 * The number of cards in a page.
 */
const PAGE_SIZE = 24;

/**
 * The list page.
 */
export function WikidataItemList({ showGraph = false }: WikidataItemListProps) {
  const [selection, setSelection] = useState<Selection>(EMPTY_SELECTION);

  // Refine owns the query: the filters, the sorters and the page are
  // its state, and the backend answers with one page.
  const {
    tableQuery,
    result,
    filters,
    setFilters,
    currentPage,
    setCurrentPage,
    pageCount,
  } = useTable<WikidataRecord>({
    resource: RESOURCE,
    pagination: { currentPage: 1, pageSize: PAGE_SIZE },
  });

  // The table wants an array it may reorder, and the server already
  // sent the order it wants.
  const records = useMemo<WikidataRecord[]>(() => [...(result.data ?? [])], [result.data]);
  const total = result.total ?? 0;

  const visibleIds = useMemo(() => records.map((record) => String(record.id)), [records]);

  const onSelect = useCallback((id: string, additive: boolean) => {
    setSelection((current) => (additive ? toggle(current, id) : { ids: new Set([id]) }));
  }, []);

  if (tableQuery.isLoading) {
    return (
      <div className="flex items-center justify-center gap-2 p-8 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        Loading the items…
      </div>
    );
  }

  if (tableQuery.isError) {
    return (
      <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-4 text-sm">
        <p className="font-medium">The items could not be loaded.</p>
        <p className="mt-1 text-muted-foreground">{tableQuery.error?.message}</p>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      <FilterBar filters={filters} onChange={setFilters} total={total} />

      <div className="flex items-center gap-2 text-xs">
        <button
          type="button"
          className="rounded-md border border-border px-2 py-1 hover:bg-accent hover:text-foreground"
          onClick={() =>
            setSelection((current) =>
              current.ids.size > 0
                ? clear()
                : selectAll(current, visibleIds),
            )
          }
        >
          {selection.ids.size > 0
            ? "Select none"
            : "Select the page"}
        </button>

        <span>{summarize(selection, visibleIds.length, total)}</span>

        {selection.ids.size > 0 ? (
          <button
            type="button"
            className="rounded-md border border-border px-2 py-1 hover:bg-accent hover:text-foreground"
            onClick={() => setSelection(clear())}
          >
            Clear
          </button>
        ) : null}

        <span className="ml-auto">{pageSummary(currentPage, PAGE_SIZE, total, visibleIds.length)}</span>
      </div>

      {records.length === 0 ? (
        <p className="rounded-lg border border-border bg-card p-6 text-center text-sm text-muted-foreground">
          No item matches the filters.
        </p>
      ) : (
        <GalleryGrid
          records={records}
          showGraph={showGraph}
          selectedIds={selection.ids}
          onSelect={onSelect}
        />
      )}

      {pageCount > 1 ? (
        <div className="flex items-center justify-center gap-2 text-xs">
          <button
            type="button"
            className="rounded-md border border-border px-2 py-1 hover:bg-accent hover:text-foreground disabled:opacity-40"
            onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
            disabled={currentPage <= 1}
          >
            Previous
          </button>
          <span className="text-muted-foreground">
            Page {currentPage} of {pageCount}
          </span>
          <button
            type="button"
            className="rounded-md border border-border px-2 py-1 hover:bg-accent hover:text-foreground disabled:opacity-40"
            onClick={() => setCurrentPage(Math.min(pageCount, currentPage + 1))}
            disabled={currentPage >= pageCount}
          >
            Next
          </button>
        </div>
      ) : null}
    </div>
  );
}
