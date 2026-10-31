/**
 * The helpers of the card gallery.
 *
 * The gallery is a grid of cards laid out in a vertical scroller. It is
 * virtualised, so only the cards that are on screen are rendered.
 *
 * The two libraries that do the work have their own modules, next to
 * this one: `gallery/table.ts` for the row model and `gallery/virtual.ts`
 * for the grid. What is left here is what neither of them decides: the
 * words the interface says about the page it is showing.
 */

/**
 * Describes the page for the interface.
 *
 * It says which items are on screen out of how many match, which is what
 * tells the reader that the filters found something.
 */
export function pageSummary(
  page: number,
  pageSize: number,
  total: number,
  shown: number,
): string {
  if (total === 0) {
    return 'No item';
  }

  if (shown === 0) {
    return `0 of ${total}`;
  }

  const first = (page - 1) * pageSize + 1;
  const last = first + shown - 1;

  return `${first}–${last} of ${total}`;
}
