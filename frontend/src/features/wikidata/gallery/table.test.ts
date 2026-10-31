import { describe, expect, it } from 'vitest';

import { CARD_COLUMN_ID, columns, labelOf } from './table';
import type { WikidataRecord } from '../provider';

/**
 * A record of the panel, with the labels it needs.
 */
function recordOf(id: string, labels?: Record<string, { language: string; value: string }>): WikidataRecord {
  return {
    id,
    item: {
      id,
      type: 'item',
      labels,
    },
  } as unknown as WikidataRecord;
}

describe('labelOf', () => {
  it('is the label in the preferred language', () => {
    const record = recordOf('Q90', {
      en: { language: 'en', value: 'Paris' },
      fr: { language: 'fr', value: 'Paris' },
    });

    expect(labelOf(record)).toBe('Paris');
  });

  it('prefers English over French', () => {
    const record = recordOf('Q90', {
      fr: { language: 'fr', value: 'Paris' },
      en: { language: 'en', value: 'Paris' },
    });

    expect(labelOf(record)).toBe('Paris');
  });

  it('falls back to French when there is no English label', () => {
    const record = recordOf('Q90', {
      fr: { language: 'fr', value: 'Paris' },
    });

    expect(labelOf(record)).toBe('Paris');
  });

  it('is the QID when the item has no label at all', () => {
    expect(labelOf(recordOf('Q90'))).toBe('Q90');
  });

  it('is the QID when the item has an empty label', () => {
    const record = recordOf('Q90', {
      en: { language: 'en', value: '' },
    });

    expect(labelOf(record)).toBe('Q90');
  });
});

describe('columns', () => {
  it('has one column, and it is the column of the card', () => {
    expect(columns).toHaveLength(1);
    expect(columns[0]?.id).toBe(CARD_COLUMN_ID);
  });

  it('holds the label of the item', () => {
    const column = columns[0];
    const record = recordOf('Q90', {
      en: { language: 'en', value: 'Paris' },
    });

    expect(column).toBeDefined();

    const accessor = column?.accessorFn;
    expect(typeof accessor).toBe('function');

    if (typeof accessor === 'function') {
      expect(accessor(record, 0)).toBe('Paris');
    }
  });
});
