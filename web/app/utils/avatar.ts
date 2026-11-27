/**
 * Initials avatars. The API picks a stable color name from the user's id
 * (profile.AvatarColor), so a renamed user keeps their color; the web turns
 * the name into Tailwind classes here. White text on the -700 shades keeps
 * contrast at WCAG AA in light and dark mode.
 */

export const AVATAR_CLASSES: Record<string, string> = {
  rose: "bg-rose-700 text-white",
  orange: "bg-orange-700 text-white",
  amber: "bg-amber-700 text-white",
  emerald: "bg-emerald-700 text-white",
  teal: "bg-teal-700 text-white",
  sky: "bg-sky-700 text-white",
  indigo: "bg-indigo-700 text-white",
  violet: "bg-violet-700 text-white",
  fuchsia: "bg-fuchsia-700 text-white",
  slate: "bg-slate-700 text-white",
};

export function avatarClass(color: string | undefined): string {
  return (color && AVATAR_CLASSES[color]) || AVATAR_CLASSES.slate!;
}

/** First character of the first and last words, uppercased; "?" when blank. */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  const first = [...words[0]!][0]!;
  const last = words.length > 1 ? [...words[words.length - 1]!][0]! : "";
  return (first + last).toUpperCase();
}
