import type { CollectionVisibility } from "~/types/api";

/**
 * Human labels for the collection visibility levels (ADR 0009). The raw
 * values never appear in the UI. The rule itself is enforced by the API
 * (api/internal/privacy); this file only describes it.
 */
export const VISIBILITY_OPTIONS: { value: CollectionVisibility; label: string; description: string }[] = [
  { value: "public", label: "Public", description: "Anyone can see your collections, even without signing in." },
  {
    value: "followers_and_friends",
    label: "Followers & friends",
    description: "People you’ve approved as followers, and your friends.",
  },
  { value: "friends_only", label: "Friends only", description: "Only your friends." },
  {
    value: "private",
    label: "Private",
    description: "Only you. Your profile is hidden from everyone else, too.",
  },
];

export function visibilityLabel(v: CollectionVisibility): string {
  return VISIBILITY_OPTIONS.find((o) => o.value === v)?.label ?? "Private";
}

/** What a visitor is told when they can't see someone's collections. */
export function hiddenCollectionsMessage(v: CollectionVisibility, name: string): string {
  switch (v) {
    case "followers_and_friends":
      return `Only ${name}’s followers and friends can see their collections.`;
    case "friends_only":
      return `Only ${name}’s friends can see their collections.`;
    default:
      return `${name} keeps their collections private.`;
  }
}

/** What the owner is told about who can see their collections. */
export function ownVisibilityNote(v: CollectionVisibility): string {
  switch (v) {
    case "public":
      return "Anyone can see your collections.";
    case "followers_and_friends":
      return "Your followers and friends can see your collections.";
    case "friends_only":
      return "Only your friends can see your collections.";
    default:
      return "Only you can see your collections and profile.";
  }
}
