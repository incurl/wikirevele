import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import {
  MinimumTextContrast,
  colourOf,
  contrast,
  parseHsl,
  toRgb,
} from './contrast';

/**
 * The stylesheet of the application, read as it is written.
 *
 * The file is read with the filesystem and not imported: the import of
 * a stylesheet as a raw string is empty in this environment.
 */
const styles = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), '..', 'styles.css'),
  'utf8',
);

/**
 * The tokens of the design system, read as they are written.
 *
 * The palette of the interface names them instead of declaring them,
 * and they are imported by the stylesheet.
 */
const tokens = readFileSync(
  join(
    dirname(fileURLToPath(import.meta.url)),
    '..',
    '..',
    'node_modules',
    '@wikimedia',
    'codex-design-tokens',
    'theme-wikimedia-ui-mode-dark.css',
  ),
  'utf8',
);

/**
 * The value of a custom property of the stylesheet.
 *
 * The stylesheet is read as it is written, so the test sees the
 * colours the interface really uses and not a copy of them.
 */
function themeValue(name: string): string {
  // The name of a custom property has dashes in it, and a dash is
  // not a word character: it has to be escaped for the pattern.
  const escaped = name.replace(/[.*+?^${}()|[\]\\-]/g, '\\$&');
  const pattern = new RegExp(`${escaped}:\\s*([^;]+);`);

  // The stylesheet is searched first, and the tokens of the design
  // system after it.
  const match = styles.match(pattern) ?? tokens.match(pattern);

  if (!match) {
    throw new Error(`the stylesheet has no ${name}`);
  }
  return match[1].trim();
}

/**
 * The declarations of a rule of the stylesheet.
 */
function ruleDeclarations(selector: string): Record<string, string> {
  // The selector of a rule is a pattern of its own: the dots and the
  // dashes of a class name have to be escaped for it.
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  // The rule is the first block after the selector, and the block
  // ends at the first closing brace.
  const match = styles.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`));

  if (!match) {
    throw new Error(`the stylesheet has no rule for ${selector}`);
  }

  // The comments are removed first: a comment holds the punctuation
  // of the declarations, and it would be read as one of them.
  const body = match[1].replace(/\/\*[\s\S]*?\*\//g, '');

  const declarations: Record<string, string> = {};
  for (const declaration of body.split(';')) {
    const separator = declaration.indexOf(':');
    if (separator < 0) {
      continue;
    }
    const property = declaration.slice(0, separator).trim();
    const value = declaration.slice(separator + 1).trim();
    if (property) {
      declarations[property] = value;
    }
  }

  return declarations;
}

/**
 * Resolves the colour of a declaration.
 *
 * A value that names a custom property is read from the stylesheet,
 * and the property it names may name another one: the palette of the
 * interface is built on the tokens of the design system, and those
 * are hex colours.
 */
function resolve(value: string, depth = 0): readonly [number, number, number] {
  if (depth > 8) {
    throw new Error(`the colour ${value} refers to itself`);
  }

  const name = value.match(/^var\(\s*(--[^)]+)\s*\)$/);

  if (name) {
    return resolve(themeValue(name[1]), depth + 1);
  }

  return colourOf(value);
}

/**
 * The contrast between the text and the background of a rule.
 */
function ruleContrast(selector: string): number {
  const declarations = ruleDeclarations(selector);

  const text = declarations['color'];
  const background = declarations['background'] ?? declarations['background-color'];

  if (!text || !background) {
    throw new Error(`the rule ${selector} has no text or no background`);
  }

  return contrast(resolve(text), resolve(background));
}

/**
 * The contrast of two custom properties of the stylesheet.
 */
function themeContrast(first: string, second: string): number {
  return contrast(resolve(themeValue(first)), resolve(themeValue(second)));
}

describe('parseHsl', () => {
  it('reads the channels of a value of the stylesheet', () => {
    expect(parseHsl('220 2% 93%')).toEqual({ h: 220, s: 2, l: 93 });
  });

  it('refuses a value that is not a colour', () => {
    expect(() => parseHsl('not a colour')).toThrow();
  });
});

describe('toRgb', () => {
  it('turns a pure white into the three channels of white', () => {
    expect(toRgb({ h: 0, s: 0, l: 100 })).toEqual([255, 255, 255]);
  });

  it('turns a pure black into the three channels of black', () => {
    expect(toRgb({ h: 0, s: 0, l: 0 })).toEqual([0, 0, 0]);
  });
});

describe('contrast', () => {
  it('is the highest between black and white', () => {
    expect(contrast([0, 0, 0], [255, 255, 255])).toBeCloseTo(21, 5);
  });

  it('is the lowest between a colour and itself', () => {
    expect(contrast([120, 120, 120], [120, 120, 120])).toBeCloseTo(1, 5);
  });

  it('does not depend on the order of the two colours', () => {
    expect(contrast([0, 0, 0], [255, 255, 255])).toBeCloseTo(
      contrast([255, 255, 255], [0, 0, 0]),
      5,
    );
  });
});

describe('the theme', () => {
  it('reads the value of a custom property', () => {
    expect(themeValue('--foreground')).toBe('220 2% 93%');
  });

  it('keeps the text of the page readable on the page', () => {
    expect(themeContrast('--foreground', '--background')).toBeGreaterThan(
      MinimumTextContrast,
    );
  });

  it('keeps the muted text readable on the page', () => {
    expect(themeContrast('--muted-foreground', '--background')).toBeGreaterThan(
      MinimumTextContrast,
    );
  });

  it('keeps the text of a card readable on the card', () => {
    expect(themeContrast('--card-foreground', '--card')).toBeGreaterThan(
      MinimumTextContrast,
    );
  });
});

describe('the primary button', () => {
  it('puts a dark text on the light accent', () => {
    // The accent is a light blue, so the text on it has to be the
    // dark one. White on the accent is a contrast of 1.9, and the
    // button reads as a blank rectangle.
    expect(ruleContrast('.btn-primary')).toBeGreaterThan(MinimumTextContrast);
  });

  it('does not put white on the accent', () => {
    const white = toRgb({ h: 0, s: 0, l: 100 });
    const accent = resolve(themeValue('--primary'));

    expect(contrast(white, accent)).toBeLessThan(MinimumTextContrast);
  });

  it('names the dark foreground of the palette', () => {
    expect(ruleDeclarations('.btn-primary')['color']).toBe('var(--primary-foreground)');
  });
});

describe('the buttons of the panels', () => {
  it('keeps the text of a default button readable', () => {
    expect(ruleContrast('.btn')).toBeGreaterThan(MinimumTextContrast);
  });

  it('keeps the text of a primary button readable', () => {
    expect(ruleContrast('.btn-primary')).toBeGreaterThan(MinimumTextContrast);
  });

  it('keeps the text of a panel header readable', () => {
    // The header is a surface of its own, so the contrast is the one
    // of its own text on its own background.
    expect(ruleContrast('.panel-header')).toBeGreaterThan(MinimumTextContrast);
  });
});

describe('the button of the element', () => {
  it('sets a colour of its own', () => {
    // The preflight of Tailwind is not used, so nothing resets the
    // native appearance of a button: without a colour and a
    // background of its own, a button keeps the colours of the
    // system, and the system paints it with the colours of its own
    // theme. The interface declares itself dark, and the engine then
    // paints the button with a dark surface and a light text.
    const declarations = ruleDeclarations('button');

    expect(declarations['color']).toBeTruthy();
    expect(declarations['background']).toBeTruthy();
  });

  it('is readable on its own background', () => {
    expect(ruleContrast('button')).toBeGreaterThan(MinimumTextContrast);
  });

  it('takes the colours of the palette', () => {
    const declarations = ruleDeclarations('button');

    expect(declarations['color']).toBe('var(--text)');
    expect(declarations['background']).toBe('var(--bg-input)');
  });
});
