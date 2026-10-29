import { describe, expect, it } from 'vitest';

import { formatZoom, SETTINGS_TABS } from './SettingsDialog';

describe('SETTINGS_TABS', () => {
  it('lists the connection and the zoom tabs, in that order', () => {
    expect(SETTINGS_TABS.map((tab) => tab.value)).toEqual(['connection', 'zoom']);
  });
});

describe('formatZoom', () => {
  it('shows the factor as a percentage', () => {
    expect(formatZoom(125)).toBe('125%');
  });

  it('clamps a factor that is outside the range', () => {
    expect(formatZoom(500)).toBe('200%');
    expect(formatZoom(10)).toBe('50%');
  });

  it('falls back to the base size for a factor that is not a number', () => {
    expect(formatZoom(Number.NaN)).toBe('100%');
  });
});
