import type { LocationQuery } from "vue-router";
import type { OverviewCategory, OverviewSort } from "~/types/api";

/**
 * URL state for the collection overview on /my. The URL is the single source
 * of truth (shareable, back-button friendly): `view`, `sort`, and `category`
 * (comma-separated slugs). Defaults are omitted so the plain page stays /my.
 */
export type OverviewView = "collections" | "items";

export interface OverviewState {
  view: OverviewView;
  sort: OverviewSort;
  categories: string[];
}

export const SORT_OPTIONS: readonly { value: OverviewSort; label: string }[] = [
  { value: "newest", label: "Newest first" },
  { value: "oldest", label: "Oldest first" },
  { value: "name_asc", label: "Name (A–Z)" },
  { value: "name_desc", label: "Name (Z–A)" },
  { value: "category", label: "Category" },
];

export const DEFAULT_OVERVIEW_STATE: OverviewState = { view: "collections", sort: "newest", categories: [] };

/** Matches the API's limit; extra slugs in a hand-edited URL are dropped. */
export const MAX_CATEGORY_FILTERS = 20;

const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

function first(v: LocationQuery[string] | undefined): string | undefined {
  const x = Array.isArray(v) ? v[0] : v;
  return typeof x === "string" ? x : undefined;
}

/** Read overview state from the route query, ignoring anything unknown. */
export function parseOverviewQuery(query: LocationQuery): OverviewState {
  const view = first(query.view) === "items" ? "items" : "collections";
  const rawSort = first(query.sort);
  const sort = SORT_OPTIONS.some((o) => o.value === rawSort) ? (rawSort as OverviewSort) : "newest";
  const raw = query.category;
  const parts = (Array.isArray(raw) ? raw : [raw])
    .filter((v): v is string => typeof v === "string")
    .flatMap((v) => v.split(","))
    .map((s) => s.trim())
    .filter((s) => SLUG.test(s));
  const categories = [...new Set(parts)].slice(0, MAX_CATEGORY_FILTERS);
  return { view, sort, categories };
}

/** The route query for `state`, keeping unrelated params (like `q`) from `base`. */
export function overviewQuery(state: OverviewState, base: LocationQuery): LocationQuery {
  const { view: _v, sort: _s, category: _c, ...rest } = base;
  const out: LocationQuery = { ...rest };
  if (state.view !== DEFAULT_OVERVIEW_STATE.view) out.view = state.view;
  if (state.sort !== DEFAULT_OVERVIEW_STATE.sort) out.sort = state.sort;
  if (state.categories.length > 0) out.category = state.categories.join(",");
  return out;
}

/** A category with nothing in it yet: pinned, or only empty collections. */
export function isEmptyCategory(c: OverviewCategory): boolean {
  return c.item_count === 0;
}

export function visibleCategories(categories: OverviewCategory[], showEmpty: boolean): OverviewCategory[] {
  return showEmpty ? categories : categories.filter((c) => !isEmptyCategory(c));
}

export function toggleSlug(list: readonly string[], slug: string): string[] {
  return list.includes(slug) ? list.filter((s) => s !== slug) : [...list, slug];
}
