/**
 * The contrast of the colours of the interface.
 *
 * The theme is dark, and a dark theme has one trap: a light surface
 * with light text on it. The accent of the palette is a light blue,
 * so a button that paints the accent and puts white on it is a light
 * surface with light text, and the button reads as a blank rectangle.
 *
 * The helpers here are the arithmetic of that: the contrast ratio of
 * two colours, and the reading of the HSL channels the theme uses.
 *
 * The helpers are pure, and they know nothing about the stylesheet:
 * the file is read by the test, which is the only place that needs
 * it, and the application does not read a file at run time.
 */

/**
 * A colour of the theme, as the channels the stylesheet declares.
 */
export interface Hsl {
  /** The hue, in degrees. */
  readonly h: number;
  /** The saturation, as a percentage. */
  readonly s: number;
  /** The lightness, as a percentage. */
  readonly l: number;
}

/**
 * The minimum contrast of a text on a surface.
 *
 * It is the ratio of the Web Content Accessibility Guidelines for the
 * text of a normal size.
 */
export const MinimumTextContrast = 4.5;

/**
 * Reads the HSL channels of a value of the stylesheet.
 *
 * The values are written as "220 2% 93%", and the percent signs are
 * not part of the number.
 */
export function parseHsl(value: string): Hsl {
  const parts = value.trim().split(/\s+/).map((part) => Number(part.replace('%', '')));

  if (parts.length !== 3 || parts.some((part) => Number.isNaN(part))) {
    throw new Error(`not an HSL value: ${value}`);
  }

  return { h: parts[0], s: parts[1], l: parts[2] };
}

/**
 * Turns the channels into the red, green and blue of the colour.
 */
export function toRgb({ h, s, l }: Hsl): readonly [number, number, number] {
  const saturation = s / 100;
  const lightness = l / 100;

  const c = (1 - Math.abs(2 * lightness - 1)) * saturation;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = lightness - c / 2;

  let r = 0;
  let g = 0;
  let b = 0;

  if (h < 60) {
    [r, g] = [c, x];
  } else if (h < 120) {
    [r, g] = [x, c];
  } else if (h < 180) {
    [g, b] = [c, x];
  } else if (h < 240) {
    [g, b] = [x, c];
  } else if (h < 300) {
    [r, b] = [x, c];
  } else {
    [r, b] = [c, x];
  }

  return [r, g, b].map((value) => Math.round((value + m) * 255)) as unknown as readonly [
    number,
    number,
    number,
  ];
}

/**
 * The relative luminance of a colour.
 */
export function luminance(rgb: readonly [number, number, number]): number {
  const [r, g, b] = rgb.map((value) => {
    const channel = value / 255;
    return channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });

  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/**
 * The contrast ratio of two colours.
 *
 * The ratio is between 1 and 21, and it is the one of the Web Content
 * Accessibility Guidelines: the lighter colour over the darker one.
 */
export function contrast(
  first: readonly [number, number, number],
  second: readonly [number, number, number],
): number {
  const [high, low] = [luminance(first), luminance(second)].sort((a, b) => b - a);
  return (high + 0.05) / (low + 0.05);
}

/**
 * Turns a colour of the stylesheet into its red, green and blue.
 *
 * A value that names a custom property is resolved by the caller,
 * because the resolution needs the stylesheet.
 */
export function colourOf(value: string): readonly [number, number, number] {
  if (value === "#fff" || value === "#ffffff") {
    return toRgb({ h: 0, s: 0, l: 100 });
  }

  if (value.startsWith("#")) {
    const hex = value.slice(1);
    const full =
      hex.length === 3
        ? hex
            .split("")
            .map((part) => part + part)
            .join("")
        : hex;
    return [
      parseInt(full.slice(0, 2), 16),
      parseInt(full.slice(2, 4), 16),
      parseInt(full.slice(4, 6), 16),
    ];
  }

  return toRgb(parseHsl(value));
}
