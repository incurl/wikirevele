import { describe, expect, it } from 'vitest';

import {
  EMPTY_SELECTION,
  clear,
  isMultiSelect,
  isSelected,
  selectAll,
  summarize,
  toggle,
} from './selection';

describe('toggle', () => {
  it('adds an item that is not selected', () => {
    const selection = toggle(EMPTY_SELECTION, 'Q90');
    expect(isSelected(selection, 'Q90')).toBe(true);
  });

  it('removes an item that is selected', () => {
    const selection = toggle(toggle(EMPTY_SELECTION, 'Q90'), 'Q90');
    expect(isSelected(selection, 'Q90')).toBe(false);
  });

  it('leaves the other items alone', () => {
    let selection = toggle(EMPTY_SELECTION, 'Q90');
    selection = toggle(selection, 'Q148');
    expect(isSelected(selection, 'Q90')).toBe(true);
    expect(isSelected(selection, 'Q148')).toBe(true);
  });

  it('does not change the selection it is given', () => {
    const before = toggle(EMPTY_SELECTION, 'Q90');
    toggle(before, 'Q148');
    expect(before.ids.size).toBe(1);
  });
});

describe('selectAll', () => {
  it('selects every item of the list', () => {
    const selection = selectAll(EMPTY_SELECTION, ['Q1', 'Q2', 'Q3']);
    expect(selection.ids.size).toBe(3);
  });

  it('keeps the items that are already selected', () => {
    // The selection can span two pages.
    const before = toggle(EMPTY_SELECTION, 'Q99');
    const selection = selectAll(before, ['Q1', 'Q2']);
    expect(isSelected(selection, 'Q99')).toBe(true);
    expect(isSelected(selection, 'Q1')).toBe(true);
  });

  it('does not select an item twice', () => {
    const before = toggle(EMPTY_SELECTION, 'Q1');
    const selection = selectAll(before, ['Q1', 'Q2']);
    expect(selection.ids.size).toBe(2);
  });
});

describe('clear', () => {
  it('empties the selection', () => {
    const selection = toggle(EMPTY_SELECTION, 'Q90');
    expect(clear().ids.size).toBe(0);
  });
});

describe('isMultiSelect', () => {
  it('is true when the command key is held', () => {
    expect(isMultiSelect({ metaKey: true, ctrlKey: false })).toBe(true);
  });

  it('is true when the control key is held', () => {
    expect(isMultiSelect({ metaKey: false, ctrlKey: true })).toBe(true);
  });

  it('is false when no modifier is held', () => {
    expect(isMultiSelect({ metaKey: false, ctrlKey: false })).toBe(false);
  });
});

describe('summarize', () => {
  it('says that nothing is selected', () => {
    expect(summarize(EMPTY_SELECTION, 24, 100)).toBe('100 items');
  });

  it('says that there is no item at all', () => {
    expect(summarize(EMPTY_SELECTION, 0, 0)).toBe('No item');
  });

  it('counts the selected items', () => {
    const selection = toggle(EMPTY_SELECTION, 'Q1');
    expect(summarize(selection, 24, 100)).toBe('1 selected');
  });

  it('says when the whole page is selected', () => {
    const selection = selectAll(EMPTY_SELECTION, ['Q1', 'Q2']);
    expect(summarize(selection, 2, 100)).toBe('2 selected (the whole page)');
  });
});
