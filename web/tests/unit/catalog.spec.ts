import { describe, expect, it } from "vitest";
import { catalogEntryPath, identifierTitle, matchLabel, midSentence } from "~/utils/catalog";

const lego = {
  catalog_identifier: "set_number",
  attribute_schema: { type: "object", properties: { set_number: { type: "string", title: "Set number" } } },
};

describe("catalog utils", () => {
  it("labels the identifier attribute from the schema, never the raw key", () => {
    expect(identifierTitle(lego)).toBe("Set number");
    expect(identifierTitle({ catalog_identifier: "catalog_id", attribute_schema: {} })).toBe("Catalog id");
    expect(identifierTitle({ attribute_schema: {} })).toBeNull();
    expect(identifierTitle(null)).toBeNull();
  });

  it("lowercases labels mid-sentence but keeps acronyms", () => {
    expect(midSentence("Set number")).toBe("set number");
    expect(midSentence("ISBN")).toBe("ISBN");
    expect(midSentence("Artist")).toBe("artist");
  });

  it("explains each kind of match in words", () => {
    expect(matchLabel("name_and_identifier", "Set number")).toBe("Same name and set number");
    expect(matchLabel("identifier", "ISBN")).toBe("Same ISBN");
    expect(matchLabel("name", null)).toBe("Same name");
    expect(matchLabel("similar_name", "Set number")).toBe("Similar name");
  });

  it("builds entry paths under the category", () => {
    expect(catalogEntryPath("lego-sets", "e1")).toBe("/categories/lego-sets/catalog/e1");
  });
});
