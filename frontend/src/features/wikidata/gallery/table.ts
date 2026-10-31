/**
 * The table of the card gallery.
 *
 * The gallery is a grid of cards, and a card is not a cell of a
 * spreadsheet. TanStack Table is here for the **row model**: it turns
 * the records the backend sent into rows that have a stable identity,
 * and it owns the sorting the user asks for.
 *
 * The division of labour is the one the whole panel follows:
 *
 * - **Refine owns the query.** The filters, the sorters and the page
 *   are its state, and they go to the backend, which answers with one
 *   page.
 * - **TanStack Table owns the rows.** It is given that page and it
 *   sorts and pages nothing, because the server already did.
 * - **TanStack Virtual owns what is on screen.** It renders the cards
 *   that are in the viewport.
 *
 * The table does not render. The gallery draws the cards itself, and
 * the table is what tells it which records there are and in which
 * order.
 */

import {
  createColumnHelper,
  getCoreRowModel,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table";

import type { WikidataRecord } from "../provider";
import { resolveTerm } from "../view";

/**
 * The column helper of the gallery.
 */
const columnHelper = createColumnHelper<WikidataRecord>();

/**
 * The identifier of the column that holds the card.
 *
 * The gallery has one column, and a whole card is in it: the gallery
 * renders cards, not cells.
 */
export const CARD_COLUMN_ID = "card";

/**
 * The label of an item, as the sorters and the table see it.
 *
 * It is the label the card shows: the panel resolves the terms in its
 * preferred languages, and the table sorts on the same text, so that
 * the order of the rows and the order of the server agree with what
 * the reader sees.
 *
 * An item without a label falls back to its QID, which is what the
 * card shows as well.
 */
export function labelOf(record: WikidataRecord): string {
  return resolveTerm(record.item?.labels).text || record.id;
}

/**
 * The columns of the gallery.
 *
 * The column of the card is a **derived** one: it holds the label of
 * the item, which is the text the reader sees and the text the
 * sorters work on. The other fields of the item are read by the card
 * itself, and not through the table.
 */
export const columns: ColumnDef<WikidataRecord, string>[] = [
  columnHelper.accessor(labelOf, {
    id: CARD_COLUMN_ID,
    header: "Item",
    cell: (info) => info.getValue(),
  }),
];

/**
 * The options of the table of the gallery.
 *
 * They are built here and not in the component, so that the wiring of
 * the two libraries is the same everywhere, and the wiring is the part
 * that is easy to get wrong.
 */
export function buildTableOptions(data: WikidataRecord[], sorting: SortingState) {
  return {
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    // The server did the sorting, the filtering and the paging: the
    // table must not do them again.
    manualSorting: true,
    manualFiltering: true,
    manualPagination: true,
    // The row of a record is the record itself, and its identity is
    // the QID. Without this the rows are identified by their
    // position, and a card that moves between two renders is a card
    // that is mounted again.
    getRowId: (record: WikidataRecord) => record.id,
    state: { sorting },
  };
}
