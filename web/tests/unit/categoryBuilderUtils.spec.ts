import { describe, it, expect } from "vitest";
import {
  addOption,
  categoryStatusLabel,
  draftProblem,
  groupFieldsBySource,
  previewSchema,
  selectionsFromCategory,
  toDraft,
} from "~/utils/categoryBuilder";
import type { Category, LibraryField } from "~/types/api";

const lego = { slug: "lego-sets", name: "Lego Sets" };
const vinyl = { slug: "vinyl-records", name: "Vinyl Records" };
const coins = { slug: "coins", name: "Coins" };

const theme: LibraryField = {
  id: "string:theme", key: "theme", title: "Theme", type: "string",
  description: "Star Wars, Technic, etc.", options: ["Star Wars", "Technic"], sources: [lego],
};
const year: LibraryField = { id: "integer:year", key: "year", title: "Year", type: "integer", options: [], sources: [coins, vinyl] };
const pieces: LibraryField = {
  id: "integer:piece_count", key: "piece_count", title: "Piece count", type: "integer", minimum: 0, options: [], sources: [lego],
};
const library = [pieces, theme, year];

describe("groupFieldsBySource", () => {
  it("groups by category, alphabetically, listing a shared field under each", () => {
    const groups = groupFieldsBySource(library, "");
    expect(groups.map((g) => g.source.name)).toEqual(["Coins", "Lego Sets", "Vinyl Records"]);
    expect(groups.find((g) => g.source.slug === "vinyl-records")?.fields).toEqual([year]);
    expect(groups.find((g) => g.source.slug === "lego-sets")?.fields.map((f) => f.title)).toEqual(["Piece count", "Theme"]);
  });

  it("searches titles, descriptions, and category names, ignoring case and spacing", () => {
    expect(groupFieldsBySource(library, "  YEAR ").map((g) => g.source.name)).toEqual(["Coins", "Vinyl Records"]);
    expect(groupFieldsBySource(library, "technic")[0]?.fields).toEqual([theme]);
    expect(groupFieldsBySource(library, "vinyl")[0]?.fields).toEqual([year]);
    expect(groupFieldsBySource(library, "nothing like this")).toEqual([]);
  });
});

describe("addOption", () => {
  it("adds a cleaned value, keeping deliberate casing", () => {
    expect(addOption(["Star Wars"], "  LEGO   Ideas ")).toEqual({ kind: "added", options: ["Star Wars", "LEGO Ideas"], value: "LEGO Ideas" });
  });

  it("reuses an existing option typed in another case instead of duplicating it", () => {
    expect(addOption(["Star Wars"], "star wars")).toEqual({ kind: "exists", options: ["Star Wars"], value: "Star Wars" });
  });

  it("refuses blanks, overlong values, and a full list", () => {
    expect(addOption([], "   ").kind).toBe("invalid");
    expect(addOption([], "x".repeat(61)).kind).toBe("invalid");
    const full = Array.from({ length: 50 }, (_, i) => `Option ${i}`);
    expect(addOption(full, "One more").kind).toBe("invalid");
  });
});

describe("previewSchema and toDraft", () => {
  const selections = [
    { field: theme, options: ["Star Wars", "Ninjago"] },
    { field: pieces, options: [] },
    { field: year, options: ["ignored"] },
  ];

  it("builds the same schema shape the API stores", () => {
    expect(previewSchema(selections)).toEqual({
      type: "object",
      additionalProperties: false,
      properties: {
        theme: { type: "string", title: "Theme", description: "Star Wars, Technic, etc.", enum: ["Star Wars", "Ninjago"] },
        piece_count: { type: "integer", title: "Piece count", minimum: 0 },
        year: { type: "integer", title: "Year" },
      },
    });
  });

  it("sends field ids, options only for text fields, and a cleaned name", () => {
    expect(toDraft({ name: "  Brick   Sets ", description: " ", selections, identifierId: "integer:year" })).toEqual({
      name: "Brick Sets",
      fields: [
        { id: "string:theme", options: ["Star Wars", "Ninjago"] },
        { id: "integer:piece_count", options: [] },
        { id: "integer:year", options: [] },
      ],
      catalog_identifier: "integer:year",
    });
  });
});

describe("selectionsFromCategory", () => {
  it("rebuilds the picked fields, keeping ones the library no longer has", () => {
    const cat: Category = {
      id: "c1", slug: "sneakers-abc123", name: "Sneakers", status: "private", mine: true,
      attribute_schema: {
        type: "object",
        properties: {
          theme: { type: "string", title: "Theme", enum: ["Nike"] },
          color: { type: "string", title: "Color" },
        },
      },
    };
    const got = selectionsFromCategory(cat, library);
    expect(got.map((s) => [s.field.id, s.options])).toEqual([["string:theme", ["Nike"]], ["string:color", []]]);
    expect(got[0]?.field.sources).toEqual([lego]);
    expect(got[1]?.field.title).toBe("Color");
  });
});

describe("draftProblem and categoryStatusLabel", () => {
  it("words problems the way the API does", () => {
    expect(draftProblem("  ", "", [{ field: year, options: [] }])).toMatch(/name/);
    expect(draftProblem("Shoes", "", [])).toBe("Pick at least one field.");
    expect(
      draftProblem("Shoes", "", [{ field: year, options: [] }, { field: { ...year, id: "string:year", type: "string" }, options: [] }]),
    ).toMatch(/two fields called/);
    expect(draftProblem("Shoes", "", [{ field: year, options: [] }])).toBeNull();
  });

  it("labels statuses without raw values, and nothing for public categories", () => {
    expect(categoryStatusLabel("private")).toBe("Private");
    expect(categoryStatusLabel("pending")).toBe("Waiting for review");
    expect(categoryStatusLabel("rejected")).toBe("Not approved");
    expect(categoryStatusLabel("approved")).toBeNull();
    expect(categoryStatusLabel(undefined)).toBeNull();
  });
});
