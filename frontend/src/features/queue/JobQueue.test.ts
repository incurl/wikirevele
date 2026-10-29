import { describe, expect, it } from 'vitest';

import type { Job } from '../../bindings';
import { canDelete, elapsed } from './JobQueue';

/**
 * Builds a job for the tests.
 */
function job(overrides: Partial<Job> = {}): Job {
  return {
    id: 'job1',
    title: 'a job',
    prompt: 'a prompt',
    params: {},
    result: {},
    status: 'pending',
    error_code: 0,
    error_message: '',
    artifact_ids: [],
    created: new Date().toISOString(),
    updated: new Date().toISOString(),
    ...overrides,
  } as Job;
}

describe('canDelete', () => {
  it('allows the removal of a job that has come to rest', () => {
    expect(canDelete('completed')).toBe(true);
    expect(canDelete('failed')).toBe(true);
    expect(canDelete('cancelled')).toBe(true);
  });

  it('refuses the removal of a job that is still running', () => {
    expect(canDelete('pending')).toBe(false);
    expect(canDelete('running')).toBe(false);
  });
});

describe('elapsed', () => {
  it('returns an empty string when there is no timestamp', () => {
    expect(elapsed('')).toBe('');
  });

  it('returns an empty string for an unparsable timestamp', () => {
    expect(elapsed('not a date')).toBe('');
  });

  it('reports the seconds for a recent job', () => {
    const created = new Date(Date.now() - 5000).toISOString();
    expect(elapsed(created)).toBe('5s');
  });

  it('reports the minutes for an older job', () => {
    const created = new Date(Date.now() - 125000).toISOString();
    expect(elapsed(created)).toBe('2m 5s');
  });

  it('never reports a negative duration', () => {
    const created = new Date(Date.now() + 60000).toISOString();
    expect(elapsed(created)).toBe('0s');
  });
});
