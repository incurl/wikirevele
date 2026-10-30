import { describe, expect, it } from 'vitest';

import {
  formatCount,
  formatLanguages,
  formatSpeed,
  isLanguageCode,
  isQID,
  isRunning,
  parseLanguages,
  statusLabel,
} from './command';

describe('parseLanguages', () => {
  it('splits the codes on the commas', () => {
    expect(parseLanguages('en,zh,fr')).toEqual(['en', 'zh', 'fr']);
  });

  it('trims the spaces', () => {
    expect(parseLanguages(' en , fr ')).toEqual(['en', 'fr']);
  });

  it('drops the empty ones', () => {
    expect(parseLanguages('en,,fr,')).toEqual(['en', 'fr']);
  });

  it('gives an empty list for an empty field', () => {
    expect(parseLanguages('')).toEqual([]);
  });
});

describe('formatLanguages', () => {
  it('writes the codes back', () => {
    expect(formatLanguages(['en', 'zh', 'fr'])).toBe('en,zh,fr');
  });

  it('round trips with the parser', () => {
    const field = 'en, zh ,fr';
    expect(formatLanguages(parseLanguages(field))).toBe('en,zh,fr');
  });
});

describe('isLanguageCode', () => {
  it('accepts the codes of the dump', () => {
    expect(isLanguageCode('en')).toBe(true);
    expect(isLanguageCode('zh')).toBe(true);
    expect(isLanguageCode('zh-hans')).toBe(true);
    expect(isLanguageCode('be-tarask')).toBe(true);
  });

  it('refuses the codes that are not ones', () => {
    expect(isLanguageCode('')).toBe(false);
    expect(isLanguageCode('EN')).toBe(false);
    expect(isLanguageCode('en fr')).toBe(false);
    expect(isLanguageCode('en-')).toBe(false);
    expect(isLanguageCode('-en')).toBe(false);
  });
});

describe('isQID', () => {
  it('accepts an item identifier', () => {
    expect(isQID('Q90')).toBe(true);
    expect(isQID('Q148')).toBe(true);
    expect(isQID('Q662548')).toBe(true);
    expect(isQID(' Q90 ')).toBe(true);
  });

  it('refuses anything else', () => {
    expect(isQID('')).toBe(false);
    expect(isQID('Paris')).toBe(false);
    expect(isQID('P131')).toBe(false);
    expect(isQID('Q')).toBe(false);
    expect(isQID('Q90A')).toBe(false);
  });
});

describe('formatCount', () => {
  it('groups the thousands', () => {
    expect(formatCount(1234567)).toBe('1,234,567');
  });

  it('leaves a small number alone', () => {
    expect(formatCount(42)).toBe('42');
  });
});

describe('formatSpeed', () => {
  it('shows a dash when there is no speed', () => {
    expect(formatSpeed(0)).toBe('—');
  });

  it('shows the decimals of a slow run', () => {
    expect(formatSpeed(0.25)).toBe('0.25 MB/s');
  });

  it('shows one decimal of a fast run', () => {
    expect(formatSpeed(17.85)).toBe('17.9 MB/s');
  });
});

describe('statusLabel', () => {
  it('names the statuses', () => {
    expect(statusLabel('running')).toBe('Running');
    expect(statusLabel('succeeded')).toBe('Succeeded');
    expect(statusLabel('cancelled')).toBe('Cancelled');
    expect(statusLabel('failed')).toBe('Failed');
  });

  it('says unknown for anything else', () => {
    expect(statusLabel('something')).toBe('Unknown');
  });
});

describe('isRunning', () => {
  it('tells whether the run is still going', () => {
    expect(isRunning('running')).toBe(true);
    expect(isRunning('succeeded')).toBe(false);
    expect(isRunning('failed')).toBe(false);
  });
});
