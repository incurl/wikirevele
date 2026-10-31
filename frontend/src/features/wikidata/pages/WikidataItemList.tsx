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
 *   nothing, because the server already did.
 * - **TanStack Virtual owns what is on screen.** It renders the visual
 *   rows of the grid that are in the viewport, so a page of a thousand
 *   cards costs the same as a page of twenty.
 *
 * The cards themselves are the ones of the panel, and they are not
 * aware of any of it.
 */

import { useCallback, useMemo, useRef, useState } from "react";
import { useTable } from "@refinedev/core";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  createColumnHelper,
  getCoreRowModel,
  useReactTable,
  type Row,
} from "@tanstack/react-table";
import { Loader2 } from "lucide-react";

import { WikidataEntityCard } from "../components/WikidataEntityCard";
import { FilterBar } from "../components/FilterBar";
import { RESOURCE } from "../provider";
import { viewOf } from "../provider";
import type { WikidataRecord } from "../provider";
import { placesOf } from "../backend";
import {
  EMPTY_SELECTION,
  clear,
  isSelected,
  selectAll,
  summarize,
  toggle,
  type Selection,
} from "../selection";
import {
  GALLERY_COLUMNS,
  GALLERY_ROW_HEIGHT,
  pageSummary,
  rowCount,
  rowItems,
} from "../gallery";

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
 * The column of the gallery.
 *
 * There is one, and it holds an item: the gallery renders the cards of
 * the items, so a column of the table is a whole card. The table is
 * here for the row model, not for a set of columns.
 */
const columnHelper = createColumnHelper<WikidataRecord>();

const columns = [
  columnHelper.display({
    id: "card",
    cell: (info) => String(info.row.original.id),
  }),
];

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

  const records = useMemo(() => result.data ?? [], [result.data]);
  const total = result.total ?? 0;

  // The table is given the records and renders nothing by itself: the
  // server has already filtered, sorted and paged them.
  const table = useReactTable({
    data: records,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    manualFiltering: true,
  });

  const rows = table.getRowModel().rows;

  // The virtualiser scrolls the visual rows of the grid, not the items.
  const scrollRef = useRef<HTMLDivElement>(null);

  const virtualizer = useVirtualizer({
    count: rowCount(rows.length, GALLERY_COLUMNS),
    getScrollElement: () => scrollRef.current,
    estimateSize: () => GALLERY_ROW_HEIGHT,
    overscan: 3,
  });

  /**
   * Selects an item, and clears the rest unless a modifier is held.
   */
  const onSelect = useCallback((id: string, additive: boolean) => {
    setSelection((current) => {
      if (additive) {
        return toggle(current, id);
      }
      if (current.ids.size === 1 && isSelected(current, id)) {
        return clear();
      }
      return { ids: new Set([id]) };
    });
  }, []);

  const visibleIds = useMemo(() => rows.map((row) => String(row.original.id)), [rows]);

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

      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <button
          type="button"
          className="rounded-md border border-border px-2 py-1 hover:bg-accent hover:text-foreground"
          onClick={() =>
            setSelection((current) =>
              current.ids.size === visibleIds.length ? clear() : selectAll(current, visibleIds),
            )
          }
          disabled={visibleIds.length === 0}
        >
          {selection.ids.size === visibleIds.length && visibleIds.length > 0
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

      {rows.length === 0 ? (
        <p className="rounded-lg border border-border bg-card p-6 text-center text-sm text-muted-foreground">
          No item matches the filters.
        </p>
      ) : (
        <div ref={scrollRef} className="min-h-0 flex-1 overflow-auto">
          <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((virtualRow) => (
              <div
                key={virtualRow.key}
                className="absolute inset-x-0 top-0 grid gap-2"
                style={{
                  height: virtualRow.size,
                  transform: `translateY(${virtualRow.start}px)`,
                  gridTemplateColumns: `repeat(${GALLERY_COLUMNS}, minmax(0, 1fr))`,
                }}
              >
                {rowItems(rows, virtualRow.index, GALLERY_COLUMNS).map((row) => (
                  <GalleryCard
                    key={row.id}
                    row={row}
                    showGraph={showGraph}
                    selected={isSelected(selection, String(row.original.id))}
                    onSelect={onSelect}
                  />
                ))}
              </div>
            ))}
          </div>
        </div>
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

/**
 * One card of the gallery, built from a row of the table.
 */
function GalleryCard({
  row,
  showGraph,
  selected,
  onSelect,
}: {
  readonly row: Row<WikidataRecord>;
  readonly showGraph: boolean;
  readonly selected: boolean;
  readonly onSelect: (id: string, additive: boolean) => void;
}) {
  const view = viewOf(row.original);
  if (!view) {
    return null;
  }

  return (
    <WikidataEntityCard
      item={view}
      places={placesOf(view)}
      showGraph={showGraph}
      selected={selected}
      onSelect={onSelect}
    />
  );
}
