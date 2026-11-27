import { collapseWhitespace } from "./text";

/**
 * Normalize a search box value before it's sent anywhere: trim and collapse
 * internal whitespace, so "  star   wars " searches for "star wars" and a
 * box holding only spaces counts as empty. The API applies the same rule
 * (never trust the client); this keeps the URL and the request tidy.
 */
export function normalizeSearchQuery(q: string | null | undefined): string {
  return collapseWhitespace(q);
}

/** How long a search box waits after the last keystroke before searching. */
export const SEARCH_DEBOUNCE_MS = 250;
