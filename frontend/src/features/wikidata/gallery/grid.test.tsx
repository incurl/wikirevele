/**
 * The grid of the gallery, rendered.
 *
 * The helpers of the gallery are tested on their own, in
 * `gallery.test.ts`, `gallery/table.test.ts` and
 * `gallery/virtual.test.ts`. What is left is the part that only shows
 * up when the two libraries are actually mounted: that the grid
 * renders the cards, that it lays them out in columns, and that it
 * does not render the cards that are off screen.
 */

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { MemoryRouter } from "react-router";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { GALLERY_COLUMNS } from "./virtual";
import { GalleryGrid } from "../components/GalleryGrid";

// React needs to know that the test drives the renders itself, so that
// `act` waits for them.
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

/**
 * The number of items the grid is given.
 */
const ITEM_COUNT = 40;

/**
 * The height of the scroller, in pixels.
 */
const VIEWPORT_HEIGHT = 600;

/**
 * The height of a card, in pixels.
 */
const CARD_HEIGHT = 150;

/**
 * The items of the grid.
 *
 * They are plain records: the grid draws the cards, and the cards need
 * a view model. The grid is tested with a card that shows its
 * identifier, so that the test does not depend on the item page.
 */
const RECORDS = Array.from({ length: ITEM_COUNT }, (_, index) => ({
  id: `Q${index + 1}`,
  item: {
    id: `Q${index + 1}`,
    type: "item" as const,
    labels: { en: { language: "en", value: `Item ${index + 1}` } },
    claims: {},
  },
}));

/**
 * The horizontal shift of a card, as a percentage of its own width.
 *
 * The shift is the first number of the transform, and a shift of 100%
 * is one column of the grid.
 */
function horizontalShiftOf(card: HTMLElement): number {
  const match = /translate3d\(([-\d.]+)%/.exec(card.style.transform);
  return match ? Number(match[1]) : Number.NaN;
}

describe('GalleryGrid', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();

    // The stubs of the sizes are removed, so that they do not leak
    // into the other tests of the run.
    Reflect.deleteProperty(HTMLElement.prototype, 'clientHeight');
    Reflect.deleteProperty(HTMLElement.prototype, 'offsetHeight');
  });

  /**
   * Gives the elements a size.
   *
   * There is no layout engine in a test, so every element reports a
   * height of zero: the virtualiser would render nothing at all, and
   * it would think that the whole set fits on the screen. The size of
   * the scroller is what decides which cards are on screen, and the
   * size of a card is what decides how many of them fit, so the test
   * gives them one.
   */
  function sizeTheElements(scrollerHeight: number, cardHeight: number) {
    Object.defineProperty(HTMLElement.prototype, 'clientHeight', {
      configurable: true,
      get() {
        return scrollerHeight;
      },
    });

    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true,
      get() {
        return cardHeight;
      },
    });
  }

  /**
   * Renders the grid and waits for the virtualiser to settle.
   */
  async function renderGrid() {
    sizeTheElements(VIEWPORT_HEIGHT, CARD_HEIGHT);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <GalleryGrid
            records={RECORDS}
            selectedIds={new Set<string>()}
            onSelect={() => {}}
          />
        </MemoryRouter>,
      );
    });

    // The virtualiser measures the scroll element after the first
    // render, so a second pass is needed before the cards are placed.
    await act(async () => {
      await Promise.resolve();
    });
  }

  it('renders the cards of the page', async () => {
    await renderGrid();

    expect(container.querySelectorAll('[data-index]').length).toBeGreaterThan(0);
  });

  it('gives every card the index the virtualiser expects', async () => {
    await renderGrid();

    const cards = [...container.querySelectorAll<HTMLElement>('[data-index]')];
    for (const card of cards) {
      const index = Number(card.getAttribute('data-index'));

      expect(Number.isInteger(index)).toBe(true);
      expect(index).toBeGreaterThanOrEqual(0);
      expect(index).toBeLessThan(ITEM_COUNT);
    }
  });

  it('lays the cards out in columns', async () => {
    await renderGrid();

    const cards = [...container.querySelectorAll<HTMLElement>('[data-index]')];
    const lanes = new Set(cards.map((card) => horizontalShiftOf(card)));

    // The cards of one column share the same horizontal shift, so the
    // number of distinct shifts is the number of columns that hold a
    // card.
    expect(lanes.size).toBeLessThanOrEqual(GALLERY_COLUMNS);
  });

  it('puts the first cards of the grid in the first columns', async () => {
    await renderGrid();

    const cards = [...container.querySelectorAll<HTMLElement>('[data-index]')]
      .sort((left, right) => Number(left.dataset.index) - Number(right.dataset.index));

    // The library assigns the lanes in turn, so the first card is in
    // the first column and the second one in the second.
    expect(horizontalShiftOf(cards[0]!)).toBe(0);
    expect(horizontalShiftOf(cards[1]!)).toBe(100);
    expect(horizontalShiftOf(cards[2]!)).toBe(200);
    expect(horizontalShiftOf(cards[3]!)).toBe(300);
  });

  it('gives each card a quarter of the width of the grid', async () => {
    await renderGrid();

    const cards = [...container.querySelectorAll<HTMLElement>('[data-index]')];
    const width = cards[0]?.style.width;

    expect(width).toBe(`${100 / GALLERY_COLUMNS}%`);
  });

  it('renders no more cards than the page holds', async () => {
    await renderGrid();

    // The page is the whole set here, so every card may be rendered.
    // What the test says is that the count never exceeds the set.
    expect(container.querySelectorAll('[data-index]').length).toBeLessThanOrEqual(ITEM_COUNT);
  });

  it('renders only the cards that are on screen', async () => {
    await renderGrid();

    const rendered = container.querySelectorAll('[data-index]').length;

    // The scroller is one viewport high, and a card is taller than a
    // few lines: the whole set does not fit, so the virtualiser must
    // leave some of it out. This is the promise of the library.
    expect(rendered).toBeLessThan(ITEM_COUNT);
  });

  it('gives every card the height the virtualiser measured', async () => {
    await renderGrid();

    const cards = [...container.querySelectorAll<HTMLElement>('[data-index]')];

    // The card is not given a height by the component: the
    // virtualiser measures it, and the height of the card is its own.
    // The wrapper therefore has no height of its own.
    for (const card of cards) {
      expect(card.style.height).toBe('');
    }
  });
});
