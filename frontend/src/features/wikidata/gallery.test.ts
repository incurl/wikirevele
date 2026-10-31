import { describe, expect, it } from 'vitest';

import { pageSummary } from './gallery';

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
