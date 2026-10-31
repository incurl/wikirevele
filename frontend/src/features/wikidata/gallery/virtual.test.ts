import { describe, expect, it } from 'vitest';

import {
  GALLERY_CARD_HEIGHT,
  GALLERY_COLUMNS,
  GALLERY_GAP,
  GALLERY_OVERSCAN,
  buildVirtualizerOptions,
} from './virtual';

describe('buildVirtualizerOptions', () => {
  const options = buildVirtualizerOptions(24, () => null, (index) => `Q${index}`);

  it('counts the cards, and not the rows', () => {
    // The lanes make the rows: the count is the number of cards.
    expect(options.count).toBe(24);
  });

  it('makes a grid of four columns', () => {
    expect(options.lanes).toBe(GALLERY_COLUMNS);
    expect(options.lanes).toBe(4);
  });

  it('estimates the height of one card', () => {
    // The lanes are the columns, so the estimate is the height of a
    // card and not the height of a row of cards.
    expect(options.estimateSize(0)).toBe(GALLERY_CARD_HEIGHT);
  });

  it('keeps the lanes where they are when a card is measured', () => {
    // A card that is measured taller must not move to another column.
    expect(options.laneAssignmentMode).toBe('estimate');
  });

  it('renders a few cards beyond the viewport', () => {
    expect(options.overscan).toBe(GALLERY_OVERSCAN);
  });

  it('leaves a space between the cards of a column', () => {
    expect(options.gap).toBe(GALLERY_GAP);
  });

  it('gives every card the identity of its row', () => {
    expect(options.getItemKey(3)).toBe('Q3');
  });

  it('reads the scroll element from the caller', () => {
    const element = document.createElement('div');
    const bound = buildVirtualizerOptions(1, () => element, () => 'Q0');

    expect(bound.getScrollElement()).toBe(element);
  });
});
