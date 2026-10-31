import { describe, expect, it } from 'vitest';

import { GALLERY_COLUMNS, pageSummary, rowCount, rowItems } from './gallery';

describe('rowCount', () => {
  it('is the number of full rows', () => {
    expect(rowCount(8, 4)).toBe(2);
  });

  it('rounds a partial row up', () => {
    expect(rowCount(9, 4)).toBe(3);
  });

  it('is zero for an empty gallery', () => {
    expect(rowCount(0, 4)).toBe(0);
  });

  it('is zero when there is no column', () => {
    expect(rowCount(8, 0)).toBe(0);
  });

  it('uses the columns of the gallery by default', () => {
    expect(rowCount(GALLERY_COLUMNS * 3)).toBe(3);
  });
});

describe('rowItems', () => {
  const items = ['a', 'b', 'c', 'd', 'e'];

  it('returns the four items of the first row', () => {
    expect(rowItems(items, 0, 4)).toEqual(['a', 'b', 'c', 'd']);
  });

  it('returns the remainder of the last row', () => {
    expect(rowItems(items, 1, 4)).toEqual(['e']);
  });

  it('returns nothing past the end', () => {
    expect(rowItems(items, 5, 4)).toEqual([]);
  });

  it('returns nothing for a negative row', () => {
    expect(rowItems(items, -1, 4)).toEqual([]);
  });
});

describe('pageSummary', () => {
  it('says that nothing matched', () => {
    expect(pageSummary(1, 24, 0, 0)).toBe('No item');
  });

  it('says which items are on screen', () => {
    expect(pageSummary(1, 24, 100, 24)).toBe('1–24 of 100');
  });

  it('counts the page it is on', () => {
    expect(pageSummary(3, 24, 100, 24)).toBe('49–72 of 100');
  });

  it('handles the last, short page', () => {
    expect(pageSummary(5, 24, 100, 4)).toBe('97–100 of 100');
  });

  it('says nothing is shown when the page is past the end', () => {
    expect(pageSummary(9, 24, 100, 0)).toBe('0 of 100');
  });
});
