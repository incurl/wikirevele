import { describe, expect, it } from 'vitest';

import { isConsoleShortcut } from './shortcuts';

/**
 * Builds a key event for the tests.
 */
function key(init: KeyboardEventInit): KeyboardEvent {
  return new KeyboardEvent('keydown', init);
}

describe('isConsoleShortcut', () => {
  it('matches Ctrl+Shift+I', () => {
    expect(isConsoleShortcut(key({ key: 'I', ctrlKey: true, shiftKey: true }))).toBe(true);
  });

  it('matches the Command key as well', () => {
    expect(isConsoleShortcut(key({ key: 'I', metaKey: true, shiftKey: true }))).toBe(true);
  });

  it('ignores the shortcut without the Shift key', () => {
    expect(isConsoleShortcut(key({ key: 'i', ctrlKey: true }))).toBe(false);
  });

  it('ignores the shortcut without a modifier', () => {
    expect(isConsoleShortcut(key({ key: 'I', shiftKey: true }))).toBe(false);
  });

  it('ignores another key', () => {
    expect(isConsoleShortcut(key({ key: 'J', ctrlKey: true, shiftKey: true }))).toBe(false);
  });
});
