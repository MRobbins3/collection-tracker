import { describe, it, expect } from "vitest";
import {
  bucketLabel,
  defaultCategory,
  emptyBucketMessage,
  firstName,
  parseBucket,
  sideSummary,
} from "~/utils/compare";

describe("compare utils", () => {
  it("parses the tab from the URL, defaulting to both", () => {
    expect(parseBucket("neither")).toBe("neither");
    expect(parseBucket("mine_only")).toBe("mine_only");
    expect(parseBucket("everything")).toBe("both");
    expect(parseBucket(undefined)).toBe("both");
  });

  it("names the other person by first name", () => {
    expect(firstName("Noah Williams")).toBe("Noah");
    expect(firstName("  Cher ")).toBe("Cher");
    expect(bucketLabel("theirs_only", "Noah Williams")).toBe("Only Noah");
    expect(bucketLabel("both", "Noah Williams")).toBe("Both of you");
  });

  it("opens the first category you both have, else the first", () => {
    const c = (slug: string, mine: number, theirs: number) => ({
      slug,
      name: slug,
      public: true,
      mine_items: mine,
      their_items: theirs,
    });
    expect(defaultCategory([c("coins", 0, 2), c("lego", 3, 1)])).toBe("lego");
    expect(defaultCategory([c("coins", 0, 2), c("lego", 3, 0)])).toBe("coins");
    expect(defaultCategory([])).toBeUndefined();
  });

  it("summarizes copies and conditions", () => {
    const s = { quantity: 1, items: 1, conditions: [], collection_id: "c", collection_name: "Shelf" };
    expect(sideSummary(s)).toBe("1 copy");
    expect(sideSummary({ ...s, quantity: 3, conditions: ["Built", "Sealed"] })).toBe("3 copies · Built, Sealed");
    expect(sideSummary({ ...s, quantity: 0 })).toBe("1 copy");
    expect(sideSummary({ ...s, quantity: 0, items: 2 })).toBe("2 copies");
  });

  it("explains empty tabs, including a category without a catalog", () => {
    const ctx = { otherName: "Noah Williams", categoryName: "Stamps", catalogEntries: 0 };
    expect(emptyBucketMessage("neither", ctx)).toContain("Stamps has no catalog yet");
    expect(emptyBucketMessage("neither", { ...ctx, catalogEntries: 5 })).toContain("everything in the Stamps catalog");
    expect(emptyBucketMessage("both", ctx)).toBe("You and Noah have nothing in common in Stamps yet.");
    expect(emptyBucketMessage("theirs_only", ctx)).toBe("You have everything Noah has in Stamps.");
  });
});
