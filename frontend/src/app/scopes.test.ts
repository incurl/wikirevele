import { describe, expect, it } from "vitest";

import { DEFAULT_SCOPE, SCOPES, isScope, scopeOfPath } from "./scopes";

describe("SCOPES", () => {
  it("starts on the workarea", () => {
    expect(SCOPES[0]?.id).toBe("workarea");
  });

  it("has no duplicate identifier", () => {
    const ids = SCOPES.map((scope) => scope.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("names every scope", () => {
    for (const scope of SCOPES) {
      expect(scope.label.length).toBeGreaterThan(0);
      expect(scope.hint.length).toBeGreaterThan(0);
    }
  });

  it("has the default scope in the list", () => {
    expect(SCOPES.some((scope) => scope.id === DEFAULT_SCOPE)).toBe(true);
  });
});

describe("scopeOfPath", () => {
  it("puts the root in the workarea", () => {
    expect(scopeOfPath("/")).toBe("workarea");
  });

  it("puts the item list in the workarea", () => {
    expect(scopeOfPath("/wikidata")).toBe("workarea");
  });

  it("puts an item page in the workarea", () => {
    expect(scopeOfPath("/wikidata/Q90")).toBe("workarea");
  });

  it("leaves an unknown path alone", () => {
    expect(scopeOfPath("/nothing")).toBeNull();
  });
});

describe("isScope", () => {
  it("accepts a scope", () => {
    expect(isScope("extract")).toBe(true);
  });

  it("refuses anything else", () => {
    expect(isScope("connection")).toBe(false);
    expect(isScope("")).toBe(false);
  });
});
