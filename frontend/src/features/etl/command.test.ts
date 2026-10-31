import { describe, expect, it } from "vitest";

import {
  formatCount,
  formatLanguages,
  formatSpeed,
  formatTargetLabels,
  formatTargets,
  isLanguageCode,
  isQID,
  isRunning,
  pairTargets,
  parseLanguages,
  parseTargetLabels,
  parseTargets,
  statusLabel,
} from "./command";

describe("parseLanguages", () => {
  it("splits the codes on the commas", () => {
    expect(parseLanguages("en,zh,fr")).toEqual(["en", "zh", "fr"]);
  });

  it("trims the spaces", () => {
    expect(parseLanguages(" en , fr ")).toEqual(["en", "fr"]);
  });

  it("drops the empty ones", () => {
    expect(parseLanguages("en,,fr,")).toEqual(["en", "fr"]);
  });

  it("gives an empty list for an empty field", () => {
    expect(parseLanguages("")).toEqual([]);
  });
});

describe("formatLanguages", () => {
  it("writes the codes back", () => {
    expect(formatLanguages(["en", "zh", "fr"])).toBe("en,zh,fr");
  });

  it("round trips with the parser", () => {
    const field = "en, zh ,fr";
    expect(formatLanguages(parseLanguages(field))).toBe("en,zh,fr");
  });
});

describe("parseTargets", () => {
  it("splits the identifiers on the commas", () => {
    expect(parseTargets("Q90,Q148")).toEqual(["Q90", "Q148"]);
  });

  it("trims the spaces", () => {
    expect(parseTargets(" Q90 , Q148 ")).toEqual(["Q90", "Q148"]);
  });

  it("drops the empty ones", () => {
    expect(parseTargets("Q90,,Q148,")).toEqual(["Q90", "Q148"]);
  });

  it("gives an empty list for an empty field", () => {
    expect(parseTargets("")).toEqual([]);
  });
});

describe("parseTargetLabels", () => {
  it("splits the names on the commas", () => {
    expect(parseTargetLabels("paris,shanghai")).toEqual(["paris", "shanghai"]);
  });

  it("trims the spaces", () => {
    expect(parseTargetLabels(" paris , shanghai ")).toEqual([
      "paris",
      "shanghai",
    ]);
  });
});

describe("pairTargets", () => {
  it("pairs the identifiers with the names", () => {
    expect(pairTargets(["Q90", "Q148"], ["paris", "shanghai"])).toEqual([
      { qid: "Q90", label: "paris" },
      { qid: "Q148", label: "shanghai" },
    ]);
  });

  it("names a target without a name after its identifier", () => {
    expect(pairTargets(["Q90", "Q148"], ["paris"])).toEqual([
      { qid: "Q90", label: "paris" },
      { qid: "Q148", label: "Q148" },
    ]);
  });

  it("gives every target a name, even without any", () => {
    expect(pairTargets(["Q90"], [])).toEqual([{ qid: "Q90", label: "Q90" }]);
  });
});

describe("formatTargets", () => {
  it("writes the identifiers back", () => {
    expect(formatTargets(["Q90", "Q148"])).toBe("Q90,Q148");
  });

  it("round trips with the parser", () => {
    const field = "Q90, Q148 ";
    expect(formatTargets(parseTargets(field))).toBe("Q90,Q148");
  });
});

describe("formatTargetLabels", () => {
  it("writes the names back", () => {
    expect(formatTargetLabels(["paris", "shanghai"])).toBe("paris,shanghai");
  });
});

describe("isLanguageCode", () => {
  it("accepts the codes of the dump", () => {
    expect(isLanguageCode("en")).toBe(true);
    expect(isLanguageCode("zh")).toBe(true);
    expect(isLanguageCode("zh-hans")).toBe(true);
    expect(isLanguageCode("be-tarask")).toBe(true);
  });

  it("refuses the codes that are not ones", () => {
    expect(isLanguageCode("")).toBe(false);
    expect(isLanguageCode("EN")).toBe(false);
    expect(isLanguageCode("en fr")).toBe(false);
    expect(isLanguageCode("en-")).toBe(false);
    expect(isLanguageCode("-en")).toBe(false);
  });
});

describe("isQID", () => {
  it("accepts an item identifier", () => {
    expect(isQID("Q90")).toBe(true);
    expect(isQID("Q148")).toBe(true);
    expect(isQID("Q662548")).toBe(true);
    expect(isQID(" Q90 ")).toBe(true);
  });

  it("refuses anything else", () => {
    expect(isQID("")).toBe(false);
    expect(isQID("Paris")).toBe(false);
    expect(isQID("P131")).toBe(false);
    expect(isQID("Q")).toBe(false);
    expect(isQID("Q90A")).toBe(false);
  });
});

describe("formatCount", () => {
  it("groups the thousands", () => {
    expect(formatCount(1234567)).toBe("1,234,567");
  });

  it("leaves a small number alone", () => {
    expect(formatCount(42)).toBe("42");
  });
});

describe("formatSpeed", () => {
  it("shows a dash when there is no speed", () => {
    expect(formatSpeed(0)).toBe("—");
  });

  it("shows the decimals of a slow run", () => {
    expect(formatSpeed(0.25)).toBe("0.25 MB/s");
  });

  it("shows one decimal of a fast run", () => {
    expect(formatSpeed(17.85)).toBe("17.9 MB/s");
  });
});

describe("statusLabel", () => {
  it("names the statuses", () => {
    expect(statusLabel("running")).toBe("Running");
    expect(statusLabel("succeeded")).toBe("Succeeded");
    expect(statusLabel("cancelled")).toBe("Cancelled");
    expect(statusLabel("failed")).toBe("Failed");
  });

  it("says unknown for anything else", () => {
    expect(statusLabel("something")).toBe("Unknown");
  });
});

describe("isRunning", () => {
  it("tells whether the run is still going", () => {
    expect(isRunning("running")).toBe(true);
    expect(isRunning("succeeded")).toBe(false);
    expect(isRunning("failed")).toBe(false);
  });
});
