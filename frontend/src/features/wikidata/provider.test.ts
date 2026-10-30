import { describe, expect, it } from "vitest";

import { claimValues, createWikidataProvider, matchesFilter, RESOURCE } from "./provider";
import { InMemorySource } from "./ndjson";
import type { WikidataItem } from "./types";

const ITEMS: WikidataItem[] = [
  {
    id: "Q1",
    type: "item",
    labels: { en: { language: "en", value: "Notre Dame" } },
    claims: {
      P131: [
        {
          mainsnak: {
            snaktype: "value",
            property: "P131",
            datatype: "wikibase-item",
            datavalue: { type: "wikibase-entityid", value: { "entity-type": "item", id: "Q90" } },
          },
        },
      ],
      P136: [
        {
          mainsnak: {
            snaktype: "value",
            property: "P136",
            datatype: "wikibase-item",
            datavalue: { type: "wikibase-entityid", value: { "entity-type": "item", id: "Q41184" } },
          },
        },
      ],
    },
  },
  {
    id: "Q2",
    type: "item",
    labels: { en: { language: "en", value: "The Louvre" } },
    claims: {
      P131: [
        {
          mainsnak: {
            snaktype: "value",
            property: "P131",
            datatype: "wikibase-item",
            datavalue: { type: "wikibase-entityid", value: { "entity-type": "item", id: "Q90" } },
          },
        },
      ],
      P106: [
        {
          mainsnak: {
            snaktype: "value",
            property: "P106",
            datatype: "wikibase-item",
            datavalue: { type: "wikibase-entityid", value: { "entity-type": "item", id: "Q166118" } },
          },
        },
      ],
    },
  },
];

describe("claimValues", () => {
  it("reads the identifiers a property points at", () => {
    expect(claimValues(ITEMS[0], "P131")).toEqual(["Q90"]);
  });

  it("is empty when the property is absent", () => {
    expect(claimValues(ITEMS[0], "P106")).toEqual([]);
  });
});

describe("matchesFilter", () => {
  it("filters on a claim", () => {
    const filter = { field: "claims.P131", operator: "eq", value: "Q90" } as const;
    expect(matchesFilter(ITEMS[0], filter)).toBe(true);
  });

  it("excludes the items that do not carry the claim", () => {
    const filter = { field: "claims.P106", operator: "eq", value: "Q166118" } as const;
    expect(matchesFilter(ITEMS[0], filter)).toBe(false);
    expect(matchesFilter(ITEMS[1], filter)).toBe(true);
  });

  it("filters on a label", () => {
    const filter = { field: "en", operator: "contains", value: "louvre" } as const;
    expect(matchesFilter(ITEMS[1], filter)).toBe(true);
    expect(matchesFilter(ITEMS[0], filter)).toBe(false);
  });

  it("combines the filters of a group", () => {
    const group = {
      operator: "and",
      value: [
        { field: "claims.P131", operator: "eq", value: "Q90" },
        { field: "claims.P106", operator: "eq", value: "Q166118" },
      ],
    } as const;
    expect(matchesFilter(ITEMS[0], group)).toBe(false);
    expect(matchesFilter(ITEMS[1], group)).toBe(true);
  });
});

describe("the data provider", () => {
  const source = new InMemorySource("test", ITEMS);

  it("lists the items", async () => {
    const provider = createWikidataProvider(source);
    const result = await provider.getList({ resource: RESOURCE });
    expect(result.total).toBe(2);
    expect(result.data).toHaveLength(2);
  });

  it("filters the items", async () => {
    const provider = createWikidataProvider(source);
    const result = await provider.getList({
      resource: RESOURCE,
      filters: [{ field: "claims.P131", operator: "eq", value: "Q90" }],
    });
    expect(result.total).toBe(2);
  });

  it("paginates", async () => {
    const provider = createWikidataProvider(source);
    const result = await provider.getList({
      resource: RESOURCE,
      pagination: { currentPage: 2, pageSize: 1 },
    });
    expect(result.total).toBe(2);
    expect(result.data).toHaveLength(1);
    expect((result.data[0] as { id: string }).id).toBe("Q2");
  });

  it("reads one item", async () => {
    const provider = createWikidataProvider(source);
    const result = await provider.getOne({ resource: RESOURCE, id: "Q1" });
    expect((result.data as { id: string }).id).toBe("Q1");
  });

  it("refuses a mutation", async () => {
    const provider = createWikidataProvider(source);
    await expect(provider.create({ resource: RESOURCE, variables: {} })).rejects.toThrow();
    await expect(provider.update({ resource: RESOURCE, id: "Q1", variables: {} })).rejects.toThrow();
    await expect(provider.deleteOne({ resource: RESOURCE, id: "Q1" })).rejects.toThrow();
  });
});

describe("the in-memory source", () => {
  it("holds the items", async () => {
    const source = new InMemorySource("test", ITEMS);
    expect(source.size).toBe(2);
    expect(await source.listItems()).toHaveLength(2);
  });

  it("reads one item", async () => {
    const source = new InMemorySource("test", ITEMS);
    expect((await source.getItem("Q1"))?.id).toBe("Q1");
    expect(await source.getItem("Q404")).toBeNull();
  });
});
