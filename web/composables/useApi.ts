/**
 * Two API URLs in play:
 *
 * - `publicBaseURL` is the URL the *browser* can reach. Safe to embed in the
 *   DOM (href, form action, etc.) since it survives SSR → client handoff.
 * - internal fetches prefer the docker-internal hostname during SSR for
 *   faster, loopback-free round-trips, then fall back to the public URL on
 *   the client. Never use this one as a DOM attribute — it would cause
 *   hydration mismatches because the two environments resolve it differently.
 */
export function useApi() {
  const config = useRuntimeConfig();

  const publicBaseURL = config.public.apiBase as string;
  const fetchBase = (import.meta.server ? (config.apiBase as string) : publicBaseURL) || publicBaseURL;

  async function get<T>(path: string, query?: Record<string, string | number | undefined>): Promise<T> {
    return await $fetch<T>(path, {
      baseURL: fetchBase,
      credentials: "include",
      query,
    });
  }

  return { get, publicBaseURL };
}
