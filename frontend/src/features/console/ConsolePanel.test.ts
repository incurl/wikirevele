import { describe, expect, it } from 'vitest';

import type { ConsoleEntry } from '../../bindings';
import {
  countErrors,
  entriesToText,
  entryToText,
  formatDetail,
  hasTextSelection,
  matchesFilter,
} from './ConsolePanel';

/**
 * Builds a console entry for the tests.
 */
function entry(overrides: Partial<ConsoleEntry> = {}): ConsoleEntry {
  return {
    id: 1,
    time: '12:00:00.000',
    level: 'info',
    source: 'api',
    message: 'a message',
    detail: '',
    ...overrides,
  } as ConsoleEntry;
}

describe('formatDetail', () => {
  it('pretty prints a valid JSON body', () => {
    const formatted = formatDetail('{"a":1}');
    expect(formatted).toContain('\n');
    expect(formatted).toContain('"a": 1');
  });

  it('shows a body that is not JSON verbatim', () => {
    expect(formatDetail('not json')).toBe('not json');
  });

  it('returns an empty string for an empty detail', () => {
    expect(formatDetail('')).toBe('');
    expect(formatDetail('   ')).toBe('');
  });
});

describe('entryToText', () => {
  it('renders the head of the entry', () => {
    const text = entryToText(entry());
    expect(text).toContain('[info]');
    expect(text).toContain('api: a message');
  });

  it('appends the detail when there is one', () => {
    const text = entryToText(entry({ detail: '{"a":1}' }));
    // The head comes first, then the pretty printed body.
    expect(text.startsWith('12:00:00.000 [info] api: a message\n')).toBe(true);
    expect(text).toContain('"a": 1');
  });

  it('appends a verbatim detail that is not JSON', () => {
    const text = entryToText(entry({ detail: 'plain body' }));
    expect(text.split('\n')).toHaveLength(2);
    expect(text).toContain('plain body');
  });
});

describe('entriesToText', () => {
  it('renders one block per entry', () => {
    const text = entriesToText([
      entry({ id: 1, message: 'first' }),
      entry({ id: 2, message: 'second' }),
    ]);
    expect(text).toContain('first');
    expect(text).toContain('second');
  });
});

describe('matchesFilter', () => {
  it('shows everything for the "all" filter', () => {
    expect(matchesFilter(entry({ level: 'error' }), 'all')).toBe(true);
    expect(matchesFilter(entry({ level: 'raw' }), 'all')).toBe(true);
  });

  it('shows the errors and the warnings for the "error" filter', () => {
    expect(matchesFilter(entry({ level: 'error' }), 'error')).toBe(true);
    expect(matchesFilter(entry({ level: 'warn' }), 'error')).toBe(true);
    expect(matchesFilter(entry({ level: 'info' }), 'error')).toBe(false);
    expect(matchesFilter(entry({ level: 'raw' }), 'error')).toBe(false);
  });

  it('shows only the raw bodies for the "raw" filter', () => {
    expect(matchesFilter(entry({ level: 'raw' }), 'raw')).toBe(true);
    expect(matchesFilter(entry({ level: 'info' }), 'raw')).toBe(false);
  });
});

describe('countErrors', () => {
  it('counts the errors and the warnings together', () => {
    const entries = [
      entry({ id: 1, level: 'error' }),
      entry({ id: 2, level: 'warn' }),
      entry({ id: 3, level: 'info' }),
    ];
    expect(countErrors(entries)).toBe(2);
  });

  it('counts nothing in an empty log', () => {
    expect(countErrors([])).toBe(0);
  });
});

describe('hasTextSelection', () => {
  it('reports no selection when nothing is selected', () => {
    window.getSelection()?.removeAllRanges();
    expect(hasTextSelection()).toBe(false);
  });
});
