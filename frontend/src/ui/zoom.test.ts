import { describe, expect, it } from 'vitest';

import { applyZoom, clampZoom, MAX_ZOOM, MIN_ZOOM } from './zoom';

describe('clampZoom', () => {
  it('keeps a factor that is already in range', () => {
    expect(clampZoom(100)).toBe(100);
    expect(clampZoom(150)).toBe(150);
  });

  it('clamps a factor that is too small', () => {
    expect(clampZoom(10)).toBe(MIN_ZOOM);
    expect(clampZoom(-20)).toBe(MIN_ZOOM);
  });

  it('clamps a factor that is too large', () => {
    expect(clampZoom(500)).toBe(MAX_ZOOM);
  });

  it('falls back to 100 for a value that is not a number', () => {
    expect(clampZoom(Number.NaN)).toBe(100);
    expect(clampZoom(Number.POSITIVE_INFINITY)).toBe(100);
  });

  it('rounds a fractional factor', () => {
    expect(clampZoom(123.4)).toBe(123);
    expect(clampZoom(123.6)).toBe(124);
  });
});

describe('applyZoom', () => {
  it('scales the root font size', () => {
    applyZoom(200);
    expect(document.documentElement.style.fontSize).toBe('26px');
  });

  it('writes the factor as a custom property', () => {
    applyZoom(150);
    expect(document.documentElement.style.getPropertyValue('--zoom')).toBe('1.5');
  });

  it('writes the factor as a data attribute', () => {
    applyZoom(75);
    expect(document.documentElement.dataset.zoom).toBe('75');
  });

  it('clamps an out of range factor before applying it', () => {
    applyZoom(1000);
    expect(document.documentElement.dataset.zoom).toBe(String(MAX_ZOOM));
  });
});
