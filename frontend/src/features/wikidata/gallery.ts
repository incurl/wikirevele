/**
 * The helpers of the card gallery.
 *
 * The gallery is a grid of cards laid out in a vertical scroller. It is
 * virtualised, so only the rows that are on screen are rendered, and the
 * rows of the virtualiser are the *visual* rows of the grid and not the
 * items: four cards share one row.
 *
 * The arithmetic of that is kept here, out of the component, so that it
 * can be tested without a DOM.
 */

/**
 * The number of cards in a row of the grid.
 *
 * The grid is fixed rather than measured: a gallery that re-lays itself
 * out on every resize is harder to keep stable, and the workarea has a
 * width that does not change much.
 */
export const GALLERY_COLUMNS = 4;

/**
 * The estimated height of a visual row, in pixels.
 *
 * The virtualiser measures the rows it renders, so this is only the
 * first guess: it is what decides how many rows are rendered before the
 * measurement of the first one arrives.
 */
export const GALLERY_ROW_HEIGHT = 148;

/**
 * The number of visual rows a number of items takes.
 */
export function rowCount(itemCount: number, columns: number = GALLERY_COLUMNS): number {
  if (itemCount <= 0 || columns <= 0) {
    return 0;
  }

  return Math.ceil(itemCount / columns);
}

/**
 * Returns the items of one visual row.
 *
 * A row that reaches past the end of the items returns the ones that are
 * there, so the last row of a page is shorter than the others.
 */
export function rowItems<T>(items: readonly T[], row: number, columns: number = GALLERY_COLUMNS): T[] {
  if (row < 0 || columns <= 0) {
    return [];
  }

  const start = row * columns;
  if (start >= items.length) {
    return [];
  }

  return items.slice(start, start + columns);
}

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
