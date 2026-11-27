/**
 * Shared normalizers for text people type (CLAUDE.md, "Normalize
 * user-entered values at write time"). The API applies the same rules and is
 * the authority (never trust the client); these keep the UI's previews and
 * instant feedback in step with it.
 */

/** Trim and collapse every run of whitespace to one space. */
export function collapseWhitespace(s: string | null | undefined): string {
  return (s ?? "").trim().split(/\s+/).filter(Boolean).join(" ");
}

/**
 * Title Case is the recommended display form, but people may keep their own
 * casing (acronyms like "LEGO", names like "McDonald"). So this only offers a
 * suggestion when the value is entirely lowercase; anything with a capital
 * letter is treated as a deliberate choice. Returns null when there's
 * nothing to suggest.
 */
export function suggestTitleCase(s: string): string | null {
  const clean = collapseWhitespace(s);
  if (!clean || clean !== clean.toLowerCase() || clean === clean.toUpperCase()) return null;
  const titled = clean
    .split(" ")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
  return titled === clean ? null : titled;
}

/** Same as the API's profile.NormalizeHandle: trim, drop one leading "@", lowercase. */
export function normalizeHandle(s: string): string {
  return s.trim().replace(/^@/, "").toLowerCase();
}

export const HANDLE_MIN = 3;
export const HANDLE_MAX = 30;

/**
 * Format problems with a normalized handle, worded exactly as the API words
 * them, or null. Reserved words and "taken" are left to the API's
 * availability check, so the list of reserved words lives in one place.
 */
export function handleProblem(h: string): string | null {
  const len = [...h].length;
  if (len < HANDLE_MIN || len > HANDLE_MAX) return "Handles are 3–30 characters.";
  if (!/^[a-z0-9_]+$/.test(h)) return "Use only lowercase letters, numbers, and underscores.";
  if (!/^[a-z]/.test(h)) return "Start your handle with a letter.";
  return null;
}
