/**
 * The virtual grid of the card gallery.
 *
 * The gallery shows a page of cards, and a page of a thousand cards
 * costs the same as a page of twenty: only the cards that are on
 * screen are rendered. TanStack Virtual is what makes that true.
 *
 * The grid is expressed with the library's own **lanes**. A lane is
 * one column of the grid, and an item is one card. The library then
 * assigns the cards to the lanes, lays them out and measures them, so
 * that the gallery does not have to know how a grid works.
 *
 * Two consequences follow, and both are the point of using lanes:
 *
 * - The height that is estimated is the height of **one card**, not
 *   the height of a row of cards.
 * - The cards are **measured** once they are rendered. A card that
 *   opens its detail panel, or that shows the graph of its
 *   neighbourhood, is taller than the others, and the grid must make
 *   room for it instead of clipping it.
 */

import { useVirtualizer, type Virtualizer } from "@tanstack/react-virtual";

/**
 * The number of cards in a row of the grid.
 *
 * The grid is fixed rather than measured: a gallery that re-lays
 * itself out on every resize is harder to keep stable, and the
 * workarea has a width that does not change much.
 */
export const GALLERY_COLUMNS = 4;

/**
 * The estimated height of a card, in pixels.
 *
 * The virtualiser measures the cards it renders, so this is only the
 * first guess: it is what decides how many cards are rendered before
 * the measurement of the first one arrives.
 */
export const GALLERY_CARD_HEIGHT = 148;

/**
 * The number of cards that are rendered beyond the viewport.
 *
 * A few cards are rendered off screen, so that a scroll does not show
 * the reader a blank space while the next ones are measured.
 */
export const GALLERY_OVERSCAN = 3;

/**
 * The space between two cards of a column, in pixels.
 *
 * The library adds it to the place of every card, so the grid keeps
 * its rhythm without the component having to add a margin.
 */
export const GALLERY_GAP = 8;

/**
 * The height of the viewport the grid assumes at first.
 *
 * The virtualiser measures the scroll element once it is on screen,
 * and that measurement is the truth. This is only the guess it makes
 * before that, so that the first cards are rendered instead of an
 * empty grid.
 */
export const GALLERY_VIEWPORT_HEIGHT = 600;

/**
 * The options of the virtual grid.
 *
 * They are built here so that the wiring of the library is in one
 * place, and so that it can be read without the component around it.
 */
export function buildVirtualizerOptions(
  count: number,
  getScrollElement: () => HTMLElement | null,
  getItemKey: (index: number) => string,
) {
  return {
    // The count is the number of **cards**, not the number of rows:
    // the lanes are what make the rows.
    count,
    getScrollElement,
    // The identity of a card is the identity of its row, so that the
    // card that moves keeps its place in the DOM.
    getItemKey,
    // The height of one card.
    estimateSize: () => GALLERY_CARD_HEIGHT,
    // The grid is four cards wide.
    lanes: GALLERY_COLUMNS,
    // The lanes are assigned by the estimate and not by the measure.
    // A card that is measured taller must not move to another column
    // when it is, because the columns of a grid do not shuffle while
    // the reader is looking at them.
    laneAssignmentMode: "estimate" as const,
    overscan: GALLERY_OVERSCAN,
    // The space between two cards of a column.
    gap: GALLERY_GAP,
    // The height the grid assumes before the scroll element has been
    // measured.
    initialRect: { width: 0, height: GALLERY_VIEWPORT_HEIGHT },
  };
}

/**
 * The virtual grid of the gallery.
 */
export function useGalleryVirtualizer(
  count: number,
  scrollRef: React.RefObject<HTMLElement | null>,
  getItemKey: (index: number) => string,
): Virtualizer<HTMLElement, Element> {
  return useVirtualizer<HTMLElement, Element>(
    buildVirtualizerOptions(count, () => scrollRef.current, getItemKey),
  );
}

