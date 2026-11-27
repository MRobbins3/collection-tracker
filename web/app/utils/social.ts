/**
 * Copy helpers for friends and followers (ADR 0014). The API decides who
 * may do what; this file only turns its answers into words.
 */

/** The API's error message for a failed action, or a plain fallback. */
export function socialErrorMessage(e: unknown): string {
  const data = (e as { data?: { error?: unknown } })?.data;
  if (typeof data?.error === "string" && data.error) return data.error;
  const status = (e as { statusCode?: number })?.statusCode ?? (e as { status?: number })?.status;
  if (status === 404) return "That person or request isn’t available any more.";
  return "Something went wrong. Please try again.";
}

/** "1 request" / "3 requests", for the badge's accessible label. */
export function requestCountLabel(n: number): string {
  return `${n} request${n === 1 ? "" : "s"} waiting`;
}
