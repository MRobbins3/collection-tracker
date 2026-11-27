import { describe, it, expect } from "vitest";
import { diffRows, kindLabel, outcomeMessage, statusLabel } from "~/utils/moderation";
import type { AttributeSchema, CatalogEntry } from "~/types/api";

const schema: AttributeSchema = {
  type: "object",
  properties: {
    set_number: { type: "string", title: "Set number" },
    piece_count: { type: "integer", title: "Piece count" },
    theme: { type: "string" },
  },
};

const entry: CatalogEntry = {
  id: "e1",
  category_id: "c1",
  category_slug: "lego-sets",
  category_name: "Lego Sets",
  name: "Millennium Falcon",
  identifier: "75192",
  attributes: { set_number: "75192", piece_count: 7541 },
  source: "seed",
  status: "approved",
};

describe("moderation labels", () => {
  it("names kinds and statuses in words", () => {
    expect(kindLabel("catalog_entry")).toBe("New entry");
    expect(kindLabel("catalog_correction")).toBe("Correction");
    expect(statusLabel("pending", "catalog_entry")).toBe("Waiting for review");
    expect(statusLabel("approved", "catalog_entry")).toBe("Added to the catalog");
    expect(statusLabel("approved", "catalog_correction")).toBe("Correction applied");
    expect(statusLabel("rejected", "catalog_correction")).toBe("Not accepted");
    expect(statusLabel("merged", "catalog_entry")).toBe("Already in the catalog");
  });

  it("explains every submit outcome", () => {
    expect(outcomeMessage({ outcome: "submitted", item_linked: false })).toContain("An admin will review");
    expect(outcomeMessage({ outcome: "already_in_catalog", entry, item_linked: true })).toBe(
      "“Millennium Falcon” is already in the catalog. Your item is now linked to it.",
    );
    expect(outcomeMessage({ outcome: "already_in_catalog", entry, item_linked: false })).toBe(
      "“Millennium Falcon” is already in the catalog.",
    );
    expect(outcomeMessage({ outcome: "already_submitted", item_linked: false })).toContain("You already suggested this");
    expect(outcomeMessage({ outcome: "pending_review", item_linked: false })).toContain("Someone already suggested this");
  });
});

describe("diffRows", () => {
  it("lists only changed fields, with human labels, before and after", () => {
    const rows = diffRows(schema, entry, {
      name: "Millennium Falcon UCS",
      attributes: { set_number: "75192", piece_count: 7500, theme: "Star Wars" },
    });
    expect(rows).toEqual([
      { key: "name", label: "Name", before: "Millennium Falcon", after: "Millennium Falcon UCS" },
      { key: "attributes.piece_count", label: "Piece count", before: "7541", after: "7500" },
      { key: "attributes.theme", label: "Theme", before: "—", after: "Star Wars" },
    ]);
  });

  it("shows a cleared description and a removed attribute", () => {
    const rows = diffRows(schema, { ...entry, description: "Big." }, { description: "", attributes: { set_number: "75192" } });
    expect(rows).toEqual([
      { key: "description", label: "Description", before: "Big.", after: "—" },
      { key: "attributes.piece_count", label: "Piece count", before: "7541", after: "—" },
    ]);
  });

  it("is empty when nothing changes", () => {
    expect(diffRows(schema, entry, { name: "Millennium Falcon" })).toEqual([]);
  });
});
