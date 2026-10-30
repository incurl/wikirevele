import { describe, expect, it } from "vitest";

import { formatTime } from "./fields/ValueRenderer";
import { parseTime, resolveTerm, resolveValue, toEntityView } from "./view";
import type { WikidataItem } from "./types";

/**
 * An item with one claim of each kind the panel renders.
 */
const ITEM: WikidataItem = {
  id: "Q2526255",
  type: "item",
  labels: { en: { language: "en", value: "The Matrix" }, fr: { language: "fr", value: "Matrix" } },
  descriptions: { en: { language: "en", value: "A film" } },
  aliases: { en: [{ language: "en", value: "Matrix" }] },
  claims: {
    P915: [
      {
        mainsnak: {
          snaktype: "value",
          property: "P915",
          datatype: "wikibase-item",
          datavalue: {
            type: "wikibase-entityid",
            value: { "entity-type": "item", id: "Q60" },
          },
        },
        rank: "preferred",
        qualifiers: {
          P580: [
            {
              snaktype: "value",
              property: "P580",
              datatype: "time",
              datavalue: {
                type: "time",
                value: { time: "+00000002014-01-01T00:00:00Z", precision: 11 },
              },
            },
          ],
        },
        "qualifiers-order": ["P580"],
        references: [{ snaks: { P248: [{ snaktype: "value", property: "P248", datatype: "wikibase-item" }] } }],
      },
    ],
    P345: [
      {
        mainsnak: {
          snaktype: "value",
          property: "P345",
          datatype: "string",
          datavalue: { type: "string", value: "tt0133093" },
        },
      },
    ],
    P625: [
      {
        mainsnak: {
          snaktype: "value",
          property: "P625",
          datatype: "globecoordinate",
          datavalue: {
            type: "globecoordinate",
            value: {
              latitude: 48.8566,
              longitude: 2.3522,
              globe: "http://www.wikidata.org/entity/Q2",
              precision: 0.0001,
            },
          },
        },
      },
    ],
    P571: [
      {
        mainsnak: {
          snaktype: "value",
          property: "P571",
          datatype: "time",
          datavalue: {
            type: "time",
            value: { time: "+1999-03-31T00:00:00Z", precision: 11 },
          },
        },
      },
    ],
    P1082: [
      {
        mainsnak: {
          snaktype: "value",
          property: "P1082",
          datatype: "quantity",
          datavalue: {
            type: "quantity",
            value: { amount: "+2138551", unit: "http://www.wikidata.org/entity/Q515" },
          },
        },
      },
    ],
    P856: [
      {
        mainsnak: {
          snaktype: "value",
          property: "P856",
          datatype: "url",
          datavalue: { type: "string", value: "http://example.org/" },
        },
      },
    ],
  },
};

describe("resolveTerm", () => {
  it("prefers the English label", () => {
    const term = resolveTerm(ITEM.labels);
    expect(term.text).toBe("The Matrix");
    expect(term.language).toBe("en");
    expect(term.languageKind).toBe("primary");
  });

  it("falls back to the French label", () => {
    const term = resolveTerm({ fr: { language: "fr", value: "Paris" } });
    expect(term.text).toBe("Paris");
    expect(term.languageKind).toBe("fallback");
  });

  it("is empty when there is no label", () => {
    const term = resolveTerm(undefined);
    expect(term.text).toBe("");
    expect(term.languageKind).toBe("none");
  });
});

describe("parseTime", () => {
  it("reads a padded year", () => {
    const time = parseTime({ time: "+00000002014-01-01T00:00:00Z", precision: 11 });
    expect(time.year).toBe(2014);
    expect(time.month).toBe(1);
    expect(time.day).toBe(1);
  });

  it("drops the month when the precision is a year", () => {
    const time = parseTime({ time: "+1999-03-31T00:00:00Z", precision: 0 });
    expect(time.year).toBe(1999);
    expect(time.month).toBe(0);
    expect(time.day).toBe(0);
  });

  it("reads a date before the common era", () => {
    const time = parseTime({ time: "-0444-03-15T00:00:00Z", precision: 11 });
    expect(time.year).toBe(-444);
  });

  it("survives a malformed value", () => {
    const time = parseTime({ time: "not a date", precision: 11 });
    expect(time.year).toBe(0);
  });
});

describe("formatTime", () => {
  it("shows only the year at a year precision", () => {
    expect(formatTime(parseTime({ time: "+1999-03-31T00:00:00Z", precision: 0 }))).toBe("1999");
  });

  it("shows the month at a month precision", () => {
    expect(formatTime(parseTime({ time: "+1999-03-31T00:00:00Z", precision: 1 }))).toBe("March 1999");
  });

  it("shows the day at a day precision", () => {
    expect(formatTime(parseTime({ time: "+1999-03-31T00:00:00Z", precision: 11 }))).toBe("31 March 1999");
  });

  it("says when the date is before the earliest known one", () => {
    const time = parseTime({ time: "+1999-03-31T00:00:00Z", precision: 0, before: 1 });
    expect(formatTime(time)).toContain("before");
  });
});

describe("resolveValue", () => {
  it("reads an entity value", () => {
    const value = resolveValue({
      snaktype: "value",
      property: "P915",
      datatype: "wikibase-item",
      datavalue: { type: "wikibase-entityid", value: { "entity-type": "item", id: "Q60" } },
    });
    expect(value).toEqual({ kind: "entity", id: "Q60", entityType: "item" });
  });

  it("tells a URL from a plain string", () => {
    const url = resolveValue({
      snaktype: "value",
      property: "P856",
      datatype: "url",
      datavalue: { type: "string", value: "http://example.org/" },
    });
    expect(url.kind).toBe("url");

    const text = resolveValue({
      snaktype: "value",
      property: "P345",
      datatype: "string",
      datavalue: { type: "string", value: "tt0133093" },
    });
    expect(text.kind).toBe("text");
  });

  it("says when a statement has no value", () => {
    expect(resolveValue({ snaktype: "novalue", property: "P31" }).kind).toBe("no-value");
    expect(resolveValue({ snaktype: "somevalue", property: "P31" }).kind).toBe("some-value");
  });
});

describe("toEntityView", () => {
  it("does not change the item", () => {
    const before = JSON.stringify(ITEM);
    toEntityView(ITEM);
    expect(JSON.stringify(ITEM)).toBe(before);
  });

  it("builds one claim per property", () => {
    const view = toEntityView(ITEM);
    expect(view.claims).toHaveLength(6);
    expect(view.claims.map((claim) => claim.propertyId)).toEqual([
      "P1082",
      "P345",
      "P571",
      "P625",
      "P856",
      "P915",
    ]);
  });

  it("keeps the qualifiers and the references", () => {
    const view = toEntityView(ITEM);
    const statement = view.claimsByProperty.get("P915")?.[0];
    expect(statement?.qualifiers).toHaveLength(1);
    expect(statement?.qualifiers[0].propertyId).toBe("P580");
    expect(statement?.references).toHaveLength(1);
  });

  it("reads the rank", () => {
    const view = toEntityView(ITEM);
    expect(view.claimsByProperty.get("P915")?.[0].rank).toBe("preferred");
  });

  it("has no image when P18 is absent", () => {
    expect(toEntityView(ITEM).imageId).toBeNull();
  });
});
