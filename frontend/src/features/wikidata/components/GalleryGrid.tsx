/**
 * The virtual grid of the card gallery.
 *
 * The grid is the part of the panel that draws the cards, and it is
 * the only part that knows about the two libraries:
 *
 * - **TanStack Table** gives the rows. It is given the records the
 *   backend sent and it renders nothing by itself: the server has
 *   already filtered, sorted and paged them.
 * - **TanStack Virtual** gives the place of every card. The grid is
 *   made of the library's lanes, so the library lays the cards out and
 *   measures them.
 *
 * The component is a plain function of its props, so that it can be
 * rendered on its own: the page that holds it does the querying, and
 * the grid does the drawing.
 */

import { useCallback, useMemo, useRef } from "react";
import { useReactTable, type Row } from "@tanstack/react-table";

import type { WikidataRecord } from "../provider";
import { viewOf } from "../provider";
import { placesOf } from "../backend";
import { buildTableOptions } from "../gallery/table";
import { GALLERY_COLUMNS, useGalleryVirtualizer } from "../gallery/virtual";

import { WikidataEntityCard } from "./WikidataEntityCard";

/**
 * The props of the grid.
 */
export interface GalleryGridProps {
  /** The records of the page, as the backend sent them. */
  readonly records: WikidataRecord[];
  /** Whether the graph is shown in the cards. */
  readonly showGraph?: boolean;
  /** The items that are selected. */
  readonly selectedIds?: ReadonlySet<string>;
  /** Called when the user selects an item. */
  readonly onSelect?: (id: string, additive: boolean) => void;
}

/**
 * The virtual grid of the gallery.
 */
export function GalleryGrid({
  records,
  showGraph = false,
  selectedIds = new Set<string>(),
  onSelect,
}: GalleryGridProps) {
  // The table owns the identity of the rows. It is given the records
  // and it sorts and pages nothing, because the server already did.
  const table = useReactTable(buildTableOptions(records, []));

  const rows = useMemo(() => table.getRowModel().rows, [table, records]);

  // The scroll container of the grid.
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // The virtualiser lays the cards out. The count is the number of
  // cards, and the lanes are what make the rows of the grid.
  const virtualizer = useGalleryVirtualizer(
    rows.length,
    scrollRef,
    (index: number) => rows[index]?.id ?? String(index),
  );

  const select = useCallback(
    (id: string, additive: boolean) => {
      onSelect?.(id, additive);
    },
    [onSelect],
  );

  // The grid is as tall as all of its cards, so that the scrollbar is
  // the one of the whole page and not of the cards that happen to be
  // rendered.
  const totalSize = virtualizer.getTotalSize();

  return (
    <div ref={scrollRef} className="min-h-0 flex-1 overflow-auto">
      <div className="relative w-full" style={{ height: totalSize }}>
        {virtualizer.getVirtualItems().map((virtualCard) => {
          const row = rows[virtualCard.index];
          if (!row) {
            return null;
          }

          return (
            <div
              key={virtualCard.key}
              data-index={virtualCard.index}
              ref={virtualizer.measureElement}
              className="absolute top-0 px-0.5"
              style={{
                // The card is placed in the lane the library chose, at
                // the place the library computed.
                //
                // The horizontal shift is a percentage of the card's
                // own width, and the card is a quarter of the grid: a
                // shift of 100% is one column. The height is left to
                // the card, so that the virtualiser can measure it.
                transform: `translate3d(${virtualCard.lane * 100}%, ${virtualCard.start}px, 0)`,
                width: `${100 / GALLERY_COLUMNS}%`,
              }}
            >
              <GalleryCard
                row={row}
                showGraph={showGraph}
                selected={selectedIds.has(String(row.original.id))}
                onSelect={select}
              />
            </div>
          );
        })}
      </div>
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
