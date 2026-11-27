import { describe, it, expect } from "vitest";
import {
  overviewQuery,
  parseOverviewQuery,
  SORT_OPTIONS,
  toggleSlug,
  visibleCategories,
} from "~/utils/overview";
import type { OverviewCategory } from "~/types/api";

const cat = (slug: string, item_count: number, extra: Partial<OverviewCategory> = {}): OverviewCategory => ({
  slug,
  name: slug,
  item_count,
  collection_count: item_count > 0 ? 1 : 0,
  pinned: false,
  ...extra,
});

describe("parseOverviewQuery", () => {
  it("defaults to the collections view, newest first, no filter", () => {
    expect(parseOverviewQuery({})).toEqual({ view: "collections", sort: "newest", categories: [] });
  });

  it("reads view, sort, and a comma-separated category list", () => {
    expect(parseOverviewQuery({ view: "items", sort: "name_desc", category: "vinyl-records,lego-sets" })).toEqual({
      view: "items",
      sort: "name_desc",
      categories: ["vinyl-records", "lego-sets"],
    });
  });

  it("ignores unknown values instead of breaking the page", () => {
    expect(parseOverviewQuery({ view: "grid", sort: "price", category: "Lego Sets,,lego-sets,lego-sets" })).toEqual({
      view: "collections",
      sort: "newest",
      categories: ["lego-sets"],
    });
  });

  it("accepts repeated category params and caps the list at 20", () => {
    const many = Array.from({ length: 25 }, (_, i) => `c${i}`);
    expect(parseOverviewQuery({ category: ["coins", "stamps"] }).categories).toEqual(["coins", "stamps"]);
    expect(parseOverviewQuery({ category: many.join(",") }).categories).toHaveLength(20);
  });
});

describe("overviewQuery", () => {
  it("omits defaults so the plain URL stays /my", () => {
    expect(overviewQuery({ view: "collections", sort: "newest", categories: [] }, {})).toEqual({});
  });

  it("writes non-defaults and keeps unrelated params like the search text", () => {
    expect(
      overviewQuery(
        { view: "items", sort: "category", categories: ["coins", "stamps"] },
        { q: "falcon", view: "collections", sort: "oldest", category: "plants" },
      ),
    ).toEqual({ q: "falcon", view: "items", sort: "category", category: "coins,stamps" });
  });

  it("round-trips through parseOverviewQuery", () => {
    const state = { view: "items" as const, sort: "oldest" as const, categories: ["vinyl-records"] };
    expect(parseOverviewQuery(overviewQuery(state, {}))).toEqual(state);
  });
});

describe("visibleCategories", () => {
  const cats = [cat("lego-sets", 3), cat("coins", 0, { pinned: true }), cat("stamps", 0, { collection_count: 1 })];

  it("shows everything when empty categories are on", () => {
    expect(visibleCategories(cats, true).map((c) => c.slug)).toEqual(["lego-sets", "coins", "stamps"]);
  });

  it("hides categories with no items when off", () => {
    expect(visibleCategories(cats, false).map((c) => c.slug)).toEqual(["lego-sets"]);
  });
});

describe("toggleSlug", () => {
  it("adds and removes a slug without mutating the input", () => {
    const list = ["coins"];
    expect(toggleSlug(list, "stamps")).toEqual(["coins", "stamps"]);
    expect(toggleSlug(list, "coins")).toEqual([]);
    expect(list).toEqual(["coins"]);
  });
});

describe("SORT_OPTIONS", () => {
  it("has a human label for every sort", () => {
    expect(SORT_OPTIONS.map((o) => o.value)).toEqual(["newest", "oldest", "name_asc", "name_desc", "category"]);
    for (const o of SORT_OPTIONS) expect(o.label).not.toMatch(/_/);
  });
});
