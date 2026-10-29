/**
 * The interface zoom.
 *
 * The application runs in a webview, so the user has no way to change
 * the browser's base font size. The zoom factor scales the whole
 * interface instead: the root font size is scaled, and the sizes in the
 * stylesheet are expressed in rem, so the entire layout follows.
 *
 * The factor is also written to the document element as a data
 * attribute and as a custom property, so the stylesheet can adjust the
 * few sizes that are not rem-based.
 */

/** The smallest zoom factor the user may choose, as a percentage. */
export const MIN_ZOOM = 50;

/** The largest zoom factor the user may choose, as a percentage. */
export const MAX_ZOOM = 200;

/** The zoom presets offered in the settings dialog. */
export const ZOOM_PRESETS = [50, 75, 100, 125, 150, 175, 200];

/**
 * Clamps a zoom factor into the supported range.
 */
export function clampZoom(zoom: number): number {
  if (!Number.isFinite(zoom)) {
    return 100;
  }
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(zoom)));
}

/**
 * Applies a zoom factor to the document.
 *
 * The root font size is scaled by the factor, and the factor itself is
 * exposed as a custom property so that the stylesheet can use it for
 * the sizes that cannot be expressed in rem.
 */
export function applyZoom(zoom: number): void {
  const factor = clampZoom(zoom);
  const root = document.documentElement;

  // The base font size is 13px, matching the unscaled stylesheet.
  root.style.fontSize = `${(13 * factor) / 100}px`;
  root.style.setProperty('--zoom', String(factor / 100));
  root.dataset.zoom = String(factor);
}
