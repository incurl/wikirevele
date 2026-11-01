import { describe, expect, it } from 'vitest';

import type { SyncCollection, SyncProgress } from '../../bindings';
import {
  DIRECTION_LABELS,
  describeProgress,
  directionLabel,
  isRunning,
  lastRun,
  needsSyncFields,
  problemsOf,
  selectedCount,
} from './sync';

/**
 * Builds a collection of the sync list.
 */
function collection(overrides: Partial<SyncCollection> = {}): SyncCollection {
  return {
    name: 'wikidata_items',
    type: 'base',
    records: 12,
    has_sync_field: true,
    selected: false,
    direction: 'push',
    include_schema: false,
    last_push: '',
    last_pull: '',
    ...overrides,
  };
}

/**
 * Builds the progress of a run.
 */
function progress(overrides: Partial<SyncProgress> = {}): SyncProgress {
  return {
    collection: '',
    direction: 'push',
    phase: 'done',
    records: 0,
    total: 0,
    finished: 0,
    running: false,
    error: '',
    over: false,
    ...overrides,
  } as SyncProgress;
}

describe('directionLabel', () => {
  it('names the three directions', () => {
    expect(directionLabel('push')).toBe('Push');
    expect(directionLabel('pull')).toBe('Pull');
    expect(directionLabel('reconcile')).toBe('Reconcile');
  });

  it('shows an unknown direction as it is', () => {
    expect(directionLabel('sideways')).toBe('sideways');
  });

  it('has a label for every direction the backend knows', () => {
    expect(Object.keys(DIRECTION_LABELS).sort()).toEqual(['pull', 'push', 'reconcile']);
  });
});

describe('selectedCount', () => {
  it('counts the selected collections', () => {
    const collections = [
      collection({ name: 'a', selected: true }),
      collection({ name: 'b', selected: false }),
      collection({ name: 'c', selected: true }),
    ];
    expect(selectedCount(collections)).toBe(2);
  });

  it('counts nothing in an empty list', () => {
    expect(selectedCount([])).toBe(0);
  });
});

describe('problemsOf', () => {
  it('accepts a set that can be run', () => {
    const collections = [collection({ selected: true })];
    expect(problemsOf(collections, 'https://ruis.example.com', 'admin@example.com', true)).toEqual(
      [],
    );
  });

  it('refuses a set without a collection', () => {
    const problems = problemsOf([], 'https://ruis.example.com', 'admin@example.com', true);
    expect(problems).toHaveLength(1);
    expect(problems[0]).toContain('collection');
  });

  it('names what is missing', () => {
    const problems = problemsOf([collection({ selected: true })], '', '', false);
    expect(problems).toHaveLength(3);
    expect(problems.join(' ')).toContain('address');
    expect(problems.join(' ')).toContain('identity');
    expect(problems.join(' ')).toContain('password');
  });

  it('does not ask for the password when one is stored', () => {
    const problems = problemsOf([collection({ selected: true })], 'https://x', 'admin@x.com', true);
    expect(problems).toEqual([]);
  });
});

describe('isRunning', () => {
  it('is false without a progress', () => {
    expect(isRunning(null)).toBe(false);
  });

  it('follows the running flag of the progress', () => {
    expect(isRunning(progress({ running: true }))).toBe(true);
    expect(isRunning(progress({ running: false }))).toBe(false);
  });
});

describe('describeProgress', () => {
  it('says that no run happened yet', () => {
    expect(describeProgress(null)).toContain('No sync');
  });

  it('names the collection and the count of a run', () => {
    const text = describeProgress(
      progress({ running: true, collection: 'wikidata_items', phase: 'push', finished: 1, total: 3 }),
    );
    expect(text).toContain('wikidata_items');
    expect(text).toContain('1/3');
  });

  it('shows the failure', () => {
    const text = describeProgress(progress({ error: 'the server refused the batch' }));
    expect(text).toContain('failed');
    expect(text).toContain('the server refused the batch');
  });

  it('says that the run is over', () => {
    const text = describeProgress(progress({ over: true, total: 2, finished: 2 }));
    expect(text).toContain('over');
    expect(text).toContain('2');
  });
});

describe('lastRun', () => {
  it('says never when a collection was never synced', () => {
    expect(lastRun('')).toBe('never');
    expect(lastRun('   ')).toBe('never');
  });

  it('shows the time of a run', () => {
    expect(lastRun('2026-01-02 03:04:05')).toBe('2026-01-02 03:04:05');
  });
});

describe('needsSyncFields', () => {
  it('is true for a collection without the fields of the sync', () => {
    expect(needsSyncFields(collection({ has_sync_field: false }))).toBe(true);
  });

  it('is false for a collection that has them', () => {
    expect(needsSyncFields(collection({ has_sync_field: true }))).toBe(false);
  });
});
