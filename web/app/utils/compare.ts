import type { CompareBucket, CompareCategory, CompareSide } from "~/types/api";

/** The comparison's tabs, in order. */
export const COMPARE_BUCKETS: CompareBucket[] = ["both", "mine_only", "theirs_only", "neither"];

/** A URL value to a bucket; anything unknown is "both". */
export function parseBucket(raw: unknown): CompareBucket {
  const s = String(raw ?? "");
  return (COMPARE_BUCKETS as string[]).includes(s) ? (s as CompareBucket) : "both";
}

/** "Noah Williams" → "Noah", for short labels like "Only Noah". */
export function firstName(displayName: string): string {
  return displayName.trim().split(/\s+/)[0] || displayName;
}

/** Tab label for a bucket, naming the other person where it helps. */
export function bucketLabel(bucket: CompareBucket, otherName: string): string {
  const first = firstName(otherName);
  return {
    both: "Both of you",
    mine_only: "Only you",
    theirs_only: `Only ${first}`,
    neither: "Neither of you",
  }[bucket];
}

/**
 * The category to open when the URL names none: the first one you both
 * have items in, else the first in the list (they come sorted by name).
 */
export function defaultCategory(categories: CompareCategory[]): string | undefined {
  return (categories.find((c) => c.mine_items > 0 && c.their_items > 0) ?? categories[0])?.slug;
}

/**
 * "3 copies · Built, Sealed": how many, and in what condition. Quantities
 * summing to 0 (only possible outside the app) fall back to the item count,
 * so an owned thing never reads "0 copies".
 */
export function sideSummary(side: CompareSide): string {
  const n = side.quantity > 0 ? side.quantity : side.items;
  const parts = [n === 1 ? "1 copy" : `${n} copies`];
  if (side.conditions.length > 0) parts.push(side.conditions.join(", "));
  return parts.join(" · ");
}

/** What an empty tab says. */
export function emptyBucketMessage(
  bucket: CompareBucket,
  ctx: { otherName: string; categoryName: string; catalogEntries: number },
): string {
  const first = firstName(ctx.otherName);
  switch (bucket) {
    case "both":
      return `You and ${first} have nothing in common in ${ctx.categoryName} yet.`;
    case "mine_only":
      return `${first} has everything you have in ${ctx.categoryName}.`;
    case "theirs_only":
      return `You have everything ${first} has in ${ctx.categoryName}.`;
    case "neither":
      return ctx.catalogEntries === 0
        ? `${ctx.categoryName} has no catalog yet. This tab lists catalog items neither of you owns, so there’s nothing to show until the catalog has entries.`
        : `Between you, you have everything in the ${ctx.categoryName} catalog.`;
  }
}
